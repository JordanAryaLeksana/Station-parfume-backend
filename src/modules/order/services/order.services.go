package services

import (
	"backend/src/config"
	cartservices "backend/src/modules/cart/services"
	"backend/src/modules/order/models"
	"backend/src/repository"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var validate = validator.New()

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

// Checkout membuat order PENDING_PAYMENT dari seluruh isi keranjang user dalam
// satu transaksi: harga & nama disalin (snapshot), stok dikurangi, keranjang dikosongkan.
func Checkout(userID uint, input *models.CheckoutRequestDTO) (*repository.Order, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	var orderID uint
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var cart repository.Cart
		if err := tx.Where("user_id = ?", userID).First(&cart).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: keranjang kosong", ErrValidation)
			}
			return err
		}
		var items []repository.CartItem
		if err := cartservices.PreloadCartItem(tx).Where("cart_id = ?", cart.ID).Order("id").Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("%w: keranjang kosong", ErrValidation)
		}

		order := repository.Order{
			OrderNo:          newOrderNo(),
			UserID:           userID,
			Status:           repository.OrderPendingPayment,
			RecipientName:    input.RecipientName,
			RecipientPhone:   input.RecipientPhone,
			ShippingProvince: input.Province,
			ShippingCity:     input.City,
			ShippingDistrict: input.District,
			ShippingPostal:   input.PostalCode,
			ShippingAddress:  input.Address,
			BuyerNote:        input.BuyerNote,
		}
		for _, it := range items {
			m := it.ProductModel
			if !cartservices.IsPurchasable(m) {
				return fmt.Errorf("%w: produk di keranjang (item %d) sudah tidak tersedia", ErrValidation, it.ID)
			}
			// Kurangi stok secara atomik; gagal kalau stok tidak cukup.
			res := tx.Model(&repository.ProductModel{}).
				Where("id = ? AND stock >= ?", m.ID, it.Quantity).
				Update("stock", gorm.Expr("stock - ?", it.Quantity))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("%w: stok %s %s tidak cukup", ErrValidation, m.Product.Name, m.VariationName())
			}
			order.Items = append(order.Items, snapshotItem(m, it.Quantity))
			order.Subtotal += m.Price * int64(it.Quantity)
			order.TotalQuantity += it.Quantity
		}
		order.Total = order.Subtotal + order.ShippingFee - order.Discount

		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("failed to create order: %w", err)
		}
		if err := tx.Where("cart_id = ?", cart.ID).Delete(&repository.CartItem{}).Error; err != nil {
			return fmt.Errorf("failed to clear cart: %w", err)
		}
		orderID = order.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return GetOrderForUser(orderID, userID)
}

func snapshotItem(m *repository.ProductModel, qty int) repository.OrderItem {
	modelID, productID := m.ID, m.ProductID
	// berat per unit: model boleh override berat produk
	weight := m.Product.WeightGram
	if m.WeightGram != nil {
		weight = *m.WeightGram
	}
	var image *string
	if len(m.Product.Images) > 0 {
		image = &m.Product.Images[0].URL
	}
	return repository.OrderItem{
		ProductModelID:    &modelID,
		ProductID:         &productID,
		ProductName:       m.Product.Name,
		VariationName:     optionalString(m.VariationName()),
		Option1Label:      optionLabel(m.Option1),
		Option2Label:      optionLabel(m.Option2),
		SKU:               m.SKU,
		ImageURL:          image,
		UnitPrice:         m.Price,
		OriginalUnitPrice: m.OriginalPrice,
		Quantity:          qty,
		Subtotal:          m.Price * int64(qty),
		WeightGram:        weight,
	}
}

func GetOrderForUser(orderID, userID uint) (*repository.Order, error) {
	var order repository.Order
	err := orderQuery(config.DB).Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("order %d: %w", orderID, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func GetOrdersByUser(userID uint) ([]repository.Order, error) {
	var orders []repository.Order
	if err := orderQuery(config.DB).Where("user_id = ?", userID).Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}
	return orders, nil
}

// CancelOrder: user hanya boleh membatalkan order miliknya yang belum dibayar.
func CancelOrder(orderID, userID uint, reason *string) error {
	return config.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockOrder(tx, orderID, &userID)
		if err != nil {
			return err
		}
		if order.Status != repository.OrderPendingPayment {
			return fmt.Errorf("%w: order dengan status %s tidak bisa dibatalkan", ErrValidation, order.Status)
		}
		return ApplyStatus(tx, order, repository.OrderCancelled, reason)
	})
}

// UpdateOrderStatus (admin): pindah status sesuai CanOrderTransition, plus kurir/resi.
func UpdateOrderStatus(orderID uint, input *models.UpdateOrderStatusDTO) error {
	if err := validate.Struct(input); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return config.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockOrder(tx, orderID, nil)
		if err != nil {
			return err
		}
		extra := map[string]any{}
		if input.Courier != nil {
			extra["courier"] = *input.Courier
		}
		if input.TrackingNumber != nil {
			extra["tracking_number"] = *input.TrackingNumber
		}
		if len(extra) > 0 {
			if err := tx.Model(order).Updates(extra).Error; err != nil {
				return err
			}
		}
		return ApplyStatus(tx, order, strings.ToUpper(input.Status), input.Reason)
	})
}

// ApplyStatus memindahkan status order (hanya maju, lihat repository.CanOrderTransition)
// dan mengisi timestamp terkait. Order yang batal/kedaluwarsa mengembalikan stok.
// Harus dipanggil di dalam transaksi dengan order yang sudah di-lock.
func ApplyStatus(tx *gorm.DB, order *repository.Order, to string, reason *string) error {
	if !repository.CanOrderTransition(order.Status, to) {
		return fmt.Errorf("%w: status %s → %s tidak diizinkan", ErrValidation, order.Status, to)
	}
	now := time.Now()
	updates := map[string]any{"status": to}
	switch to {
	case repository.OrderPaid:
		updates["paid_at"] = now
	case repository.OrderShipped:
		updates["shipped_at"] = now
	case repository.OrderCompleted:
		updates["completed_at"] = now
	case repository.OrderCancelled, repository.OrderExpired:
		updates["cancelled_at"] = now
		if reason != nil {
			updates["cancel_reason"] = *reason
		}
		if err := restoreStock(tx, order.ID); err != nil {
			return err
		}
	}
	if err := tx.Model(order).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	order.Status = to
	return nil
}

func restoreStock(tx *gorm.DB, orderID uint) error {
	var items []repository.OrderItem
	if err := tx.Where("order_id = ? AND product_model_id IS NOT NULL", orderID).Find(&items).Error; err != nil {
		return err
	}
	for _, it := range items {
		if err := tx.Model(&repository.ProductModel{}).Where("id = ?", *it.ProductModelID).
			Update("stock", gorm.Expr("stock + ?", it.Quantity)).Error; err != nil {
			return fmt.Errorf("failed to restore stock: %w", err)
		}
	}
	return nil
}

// LockOrder mengunci baris order (SELECT ... FOR UPDATE). userID nil = tanpa cek pemilik.
func LockOrder(tx *gorm.DB, orderID uint, userID *uint) (*repository.Order, error) {
	return lockOrder(tx, orderID, userID)
}

func lockOrder(tx *gorm.DB, orderID uint, userID *uint) (*repository.Order, error) {
	q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", orderID)
	if userID != nil {
		q = q.Where("user_id = ?", *userID)
	}
	var order repository.Order
	if err := q.First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("order %d: %w", orderID, ErrNotFound)
		}
		return nil, err
	}
	return &order, nil
}

func orderQuery(db *gorm.DB) *gorm.DB {
	return db.Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("id") }).
		Preload("Payments", func(db *gorm.DB) *gorm.DB { return db.Order("id") })
}

// newOrderNo: "SP" + yymmdd + 8 karakter acak, mis. SP261106A1B2C3D4.
func newOrderNo() string {
	r := strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:8]
	return "SP" + time.Now().Format("060102") + r
}

func optionLabel(o *repository.ProductVariationOption) *string {
	if o == nil {
		return nil
	}
	return &o.Label
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
