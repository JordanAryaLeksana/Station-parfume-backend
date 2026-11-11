package services

import (
	"backend/src/catalog"
	"backend/src/config"
	cartservices "backend/src/modules/cart/services"
	"backend/src/modules/order/models"
	"backend/src/repository"
	"errors"
	"fmt"
	"slices"
	"strconv"
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
	ErrForbidden  = errors.New("forbidden")
)

// PaymentWindow: batas waktu bayar setelah checkout. Lewat dari ini order
// EXPIRED dan stok yang di-reserve dilepas (lihat ExpireOverdueOrders).
const PaymentWindow = 24 * time.Hour

// Checkout membuat order PENDING_PAYMENT dari seluruh isi keranjang user dalam
// satu transaksi: harga & nama disalin (snapshot), stok di-RESERVE (belum
// dipotong; dipotong saat PAID), keranjang dikosongkan.
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
		// Urut variant_id: semua checkout mengunci baris stok dengan urutan sama → tidak deadlock.
		if err := cartservices.PreloadCartItem(tx).Where("cart_id = ?", cart.ID).Order("variant_id").Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("%w: keranjang kosong", ErrValidation)
		}
		expires := time.Now().Add(PaymentWindow)
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
			ExpiresAt:        &expires,
		}
		for _, it := range items {
			v := it.Variant
			if !catalog.Purchasable(v) {
				return fmt.Errorf("%w: produk di keranjang (item %d) sudah tidak tersedia", ErrValidation, it.ID)
			}
			// Stok hanya di-reserve untuk produk yang stoknya dihitung (parfum diracik → selalu tersedia).
			if v.Product.TrackStock {
				if err := reserve(tx, v.ID, it.Quantity); err != nil {
					return fmt.Errorf("%w: %s %s", err, v.Product.Name, v.DisplayName())
				}
			}
			item := snapshotItem(v, it.Quantity)
			order.Items = append(order.Items, item)
			order.Subtotal += item.Subtotal
			order.TotalQuantity += it.Quantity
		}
		order.Total = order.Subtotal + order.ShippingFee // ongkir ditanggung toko → 0

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

// lockStock mengunci baris varian (SELECT ... FOR UPDATE) dan membaca stoknya.
func lockStock(tx *gorm.DB, variantID uint) (*repository.Variant, error) {
	var v repository.Variant
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "stock", "reserved").First(&v, variantID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &v, err
}

func reserve(tx *gorm.DB, variantID uint, qty int) error {
	v, err := lockStock(tx, variantID)
	if err != nil {
		return err
	}
	if v == nil || v.Available() < qty {
		return fmt.Errorf("%w: stok tidak cukup", ErrValidation)
	}
	return tx.Model(&repository.Variant{}).Where("id = ?", variantID).
		Updates(map[string]any{"reserved": gorm.Expr("reserved + ?", qty), "updated_at": gorm.Expr("now()")}).Error
}

func snapshotItem(v *repository.Variant, qty int) repository.OrderItem {
	variantID, productID := v.ID, v.ProductID
	price, _ := catalog.ListPrice(v) // Purchasable sudah memastikan harga bisa dihitung
	unitPrice := price.Total
	item := repository.OrderItem{
		VariantID:     &variantID,
		ProductID:     &productID,
		ProductName:   v.Product.Name,
		Grade:         v.Product.Grade,
		Concentration: v.Concentration,
		VariantLabel:  v.Label,
		VariantName:   optionalString(v.DisplayName()),
		VariantCode:   v.Code,
		UnitPrice:     unitPrice,
		PricePerML:    price.PerML,
		BottlePrice:   price.BottlePrice,
		Quantity:      qty,
		Subtotal:      unitPrice * int64(qty),
		WeightGram:    v.ShippingWeight(),
		TrackStock:    v.Product.TrackStock,
	}
	if v.Scent != nil {
		item.ScentName = &v.Scent.Name
	}
	if v.Size != nil {
		item.SizeLabel, item.VolumeML = &v.Size.Label, &v.Size.VolumeML
	}
	if len(v.Product.Images) > 0 {
		item.ImageURL = &v.Product.Images[0].URL
	}
	return item
}

// FindOrderForUser mencari pesanan milik user lewat ID angka atau nomor
// pesanan ("SP261111A1B2C3D4", tidak peka huruf besar/kecil). Halaman sukses
// Xendit membawa ?order=<nomor pesanan>.
func FindOrderForUser(ref string, userID uint) (*repository.Order, error) {
	ref = strings.TrimSpace(ref)
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil && id > 0 {
		return GetOrderForUser(uint(id), userID)
	}
	if ref == "" || len(ref) > 32 {
		return nil, fmt.Errorf("order %q: %w", ref, ErrNotFound)
	}
	var order repository.Order
	err := orderQuery(config.DB).Where("order_no = ? AND user_id = ?", strings.ToUpper(ref), userID).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("order %q: %w", ref, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
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

// staffStatusRoles: status yang boleh diset staf lewat PUT /orders/:id/status,
// dan peran yang boleh. PAID & EXPIRED hanya dari sistem (Xendit / job
// kedaluwarsa): menandai lunas manual memotong stok padahal link bayar Xendit
// masih hidup → risiko pembayaran ganda.
var staffStatusRoles = map[string][]string{
	repository.OrderProcessing: {repository.RoleOwner, repository.RoleAdmin, repository.RoleWarehouse},
	repository.OrderShipped:    {repository.RoleOwner, repository.RoleAdmin, repository.RoleWarehouse},
	repository.OrderCompleted:  {repository.RoleOwner, repository.RoleAdmin, repository.RoleWarehouse},
	repository.OrderCancelled:  {repository.RoleOwner, repository.RoleAdmin},
	repository.OrderRefunded:   {repository.RoleOwner, repository.RoleAdmin},
}

// UpdateOrderStatus (staf): pindah status sesuai CanOrderTransition dan peran.
// Mengembalikan status tujuan (huruf besar). Pembatalan oleh staf: pemanggil
// (controller) wajib mematikan sesi bayar Xendit yang masih aktif.
func UpdateOrderStatus(orderID uint, role string, input *models.UpdateOrderStatusDTO) (string, error) {
	if err := validate.Struct(input); err != nil {
		return "", fmt.Errorf("%w: %v", ErrValidation, err)
	}
	to := strings.ToUpper(strings.TrimSpace(input.Status))
	if to == repository.OrderPaid || to == repository.OrderExpired {
		return "", fmt.Errorf("%w: status %s hanya diubah otomatis oleh sistem pembayaran", ErrValidation, to)
	}
	roles, ok := staffStatusRoles[to]
	if !ok {
		return "", fmt.Errorf("%w: status %q tidak dikenal", ErrValidation, input.Status)
	}
	if !slices.Contains(roles, role) {
		return "", fmt.Errorf("%w: peran %s tidak boleh mengubah status ke %s", ErrForbidden, role, to)
	}
	return to, config.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockOrder(tx, orderID, nil)
		if err != nil {
			return err
		}
		return ApplyStatus(tx, order, to, input.Reason)
	})
}

// ApplyStatus memindahkan status order (hanya maju, lihat repository.CanOrderTransition)
// dan mengisi timestamp terkait. Stok: PAID memotong stok yang di-reserve
// (tercatat di stock_movements); CANCELLED/EXPIRED dari PENDING_PAYMENT
// melepas reservasi. REFUNDED tidak mengubah stok (barang kembali dicatat
// admin lewat penyesuaian stok "return").
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
		if err := commitStock(tx, order, true); err != nil {
			return err
		}
	case repository.OrderShipped:
		updates["shipped_at"] = now
	case repository.OrderCompleted:
		updates["completed_at"] = now
	case repository.OrderRefunded:
		// Uang dikembalikan manual lewat Dashboard Xendit; catatan pembayarannya ikut ditandai.
		if err := tx.Model(&repository.Payment{}).Where("order_id = ? AND status = ?", order.ID, repository.PaymentPaid).
			Update("status", repository.PaymentRefunded).Error; err != nil {
			return fmt.Errorf("failed to mark payment refunded: %w", err)
		}
	case repository.OrderCancelled, repository.OrderExpired:
		updates["cancelled_at"] = now
		if reason != nil {
			updates["cancel_reason"] = *reason
		}
		if order.Status == repository.OrderPendingPayment {
			if err := releaseStock(tx, order); err != nil {
				return err
			}
		}
	}
	if err := tx.Model(order).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	order.Status = to
	return nil
}

func orderItemsWithVariant(tx *gorm.DB, orderID uint) ([]repository.OrderItem, error) {
	var items []repository.OrderItem
	// Hanya item yang stoknya di-reserve saat checkout (snapshot track_stock).
	err := tx.Where("order_id = ? AND variant_id IS NOT NULL AND track_stock", orderID).Order("variant_id").Find(&items).Error
	return items, err
}

// ApplyLatePayment: pembayaran sukses datang untuk order yang sudah EXPIRED/CANCELLED
// (mis. dibayar tepat saat batas waktu habis). Uang sudah diterima, jadi order
// diterima sebagai PAID. Reservasi stoknya sudah dilepas, jadi stok dipotong
// TANPA menyentuh reserved (milik order lain). Hanya dipanggil dari jalur pembayaran
// yang sudah diverifikasi ke Xendit — admin tetap memakai ApplyStatus.
func ApplyLatePayment(tx *gorm.DB, order *repository.Order) error {
	if order.Status != repository.OrderExpired && order.Status != repository.OrderCancelled {
		return fmt.Errorf("%w: pembayaran telat hanya untuk order EXPIRED/CANCELLED (status %s)", ErrValidation, order.Status)
	}
	if err := commitStock(tx, order, false); err != nil {
		return err
	}
	if err := tx.Model(order).Updates(map[string]any{"status": repository.OrderPaid, "paid_at": time.Now()}).Error; err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	order.Status = repository.OrderPaid
	return nil
}

// commitStock: order dibayar → stok dikurangi (dicatat sebagai "sale"). reserved
// ikut dikurangi kalau stok order ini masih di-reserve (fromReservation).
func commitStock(tx *gorm.DB, order *repository.Order, fromReservation bool) error {
	items, err := orderItemsWithVariant(tx, order.ID)
	if err != nil {
		return err
	}
	for _, it := range items {
		v, err := lockStock(tx, *it.VariantID)
		if err != nil {
			return err
		}
		if v == nil {
			continue
		}
		// Pesanan yang sudah dibayar tetap dipenuhi; kalau admin sudah menurunkan
		// stok di bawah jumlah ini, stok berhenti di 0 (perlu dicek manual).
		stock := max(v.Stock-it.Quantity, 0)
		reserved := v.Reserved
		if fromReservation {
			reserved = max(v.Reserved-it.Quantity, 0)
		}
		if err := tx.Model(&repository.Variant{}).Where("id = ?", v.ID).
			Updates(map[string]any{"stock": stock, "reserved": reserved, "updated_at": gorm.Expr("now()")}).Error; err != nil {
			return err
		}
		if delta := stock - v.Stock; delta != 0 {
			oid := order.ID
			m := repository.StockMovement{VariantID: v.ID, Delta: delta, StockAfter: stock, Reason: repository.MovementSale, OrderID: &oid}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// releaseStock: order batal/kedaluwarsa sebelum dibayar → reservasi dilepas.
func releaseStock(tx *gorm.DB, order *repository.Order) error {
	items, err := orderItemsWithVariant(tx, order.ID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := tx.Model(&repository.Variant{}).Where("id = ?", *it.VariantID).
			Updates(map[string]any{"reserved": gorm.Expr("GREATEST(reserved - ?, 0)", it.Quantity), "updated_at": gorm.Expr("now()")}).Error; err != nil {
			return fmt.Errorf("failed to release stock: %w", err)
		}
	}
	return nil
}

// OverdueOrderIDs: order PENDING_PAYMENT yang sudah lewat expires_at.
func OverdueOrderIDs(now time.Time) ([]uint, error) {
	var ids []uint
	err := config.DB.Model(&repository.Order{}).
		Where("status = ? AND expires_at < ?", repository.OrderPendingPayment, now).Order("id").Pluck("id", &ids).Error
	return ids, err
}

// ExpireOrder menandai 1 order PENDING_PAYMENT sebagai EXPIRED (reservasi stok
// dilepas). false = order sudah tidak PENDING_PAYMENT (dibayar/dibatalkan duluan).
func ExpireOrder(orderID uint) (bool, error) {
	expired := false
	reason := "batas waktu pembayaran habis"
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		order, err := lockOrder(tx, orderID, nil)
		if err != nil {
			return err
		}
		if order.Status != repository.OrderPendingPayment {
			return nil
		}
		expired = true
		return ApplyStatus(tx, order, repository.OrderExpired, &reason)
	})
	return expired, err
}

// ExpireOverdueOrders menandai semua order yang lewat batas bayar sebagai EXPIRED.
// Catatan: modul payment memakai OverdueOrderIDs + ExpireOrder supaya sesi Xendit
// dicek & dibatalkan dulu (lihat payment services ReconcileAndExpire).
func ExpireOverdueOrders(now time.Time) (int, error) {
	ids, err := OverdueOrderIDs(now)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		ok, err := ExpireOrder(id)
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
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

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
