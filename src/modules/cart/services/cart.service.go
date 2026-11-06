package services

import (
	"backend/src/config"
	"backend/src/modules/cart/models"
	"backend/src/repository"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

var validate = validator.New()

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

// PreloadCartItem memuat semua yang dibutuhkan untuk menampilkan / checkout item keranjang.
func PreloadCartItem(db *gorm.DB) *gorm.DB {
	return db.Preload("ProductModel").
		Preload("ProductModel.Product").
		Preload("ProductModel.Product.Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order") }).
		Preload("ProductModel.Option1").
		Preload("ProductModel.Option2")
}

// IsPurchasable: model aktif, belum dihapus, dan produknya sedang dijual.
func IsPurchasable(m *repository.ProductModel) bool {
	return m != nil && m.IsActive && m.Product != nil && m.Product.Status == repository.ProductStatusActive
}

func GetCart(userID uint) (*models.CartResponseDTO, error) {
	var cart repository.Cart
	err := config.DB.Where("user_id = ?", userID).First(&cart).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.CartResponseDTO{Items: []models.CartItemResponseDTO{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cart: %w", err)
	}
	var items []repository.CartItem
	if err := PreloadCartItem(config.DB).Where("cart_id = ?", cart.ID).Order("id").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to get cart items: %w", err)
	}
	resp := &models.CartResponseDTO{ID: cart.ID, Items: make([]models.CartItemResponseDTO, 0, len(items))}
	for _, it := range items {
		dto := models.CartItemResponseDTO{ID: it.ID, ProductModelID: it.ProductModelID, Quantity: it.Quantity}
		if m := it.ProductModel; m != nil {
			dto.ProductID = m.ProductID
			dto.VariationName = m.VariationName()
			dto.SKU = m.SKU
			dto.UnitPrice = m.Price
			dto.OriginalPrice = m.OriginalPrice
			dto.Stock = m.Stock
			dto.Subtotal = m.Price * int64(it.Quantity)
			if m.Product != nil {
				dto.ProductName = m.Product.Name
			}
			dto.Available = IsPurchasable(m) && m.Stock >= it.Quantity
		}
		if dto.Available {
			resp.TotalQuantity += it.Quantity
			resp.TotalPrice += dto.Subtotal
		}
		resp.Items = append(resp.Items, dto)
	}
	return resp, nil
}

// AddItem menambah model ke keranjang user (keranjang dibuat otomatis).
// Kalau model sudah ada di keranjang, jumlahnya ditambahkan.
func AddItem(userID uint, input *models.AddCartItemDTO) (*models.CartResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var m repository.ProductModel
		if err := tx.Preload("Product").First(&m, input.ProductModelID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("product model %d: %w", input.ProductModelID, ErrNotFound)
			}
			return err
		}
		if !IsPurchasable(&m) {
			return fmt.Errorf("%w: produk tidak tersedia", ErrValidation)
		}
		cart, err := getOrCreateCart(tx, userID)
		if err != nil {
			return err
		}
		var item repository.CartItem
		err = tx.Where("cart_id = ? AND product_model_id = ?", cart.ID, m.ID).First(&item).Error
		switch {
		case err == nil:
			qty := item.Quantity + input.Quantity
			if qty > m.Stock {
				return fmt.Errorf("%w: stok tersisa %d", ErrValidation, m.Stock)
			}
			return tx.Model(&item).Update("quantity", qty).Error
		case errors.Is(err, gorm.ErrRecordNotFound):
			if input.Quantity > m.Stock {
				return fmt.Errorf("%w: stok tersisa %d", ErrValidation, m.Stock)
			}
			return tx.Create(&repository.CartItem{CartID: cart.ID, ProductModelID: m.ID, Quantity: input.Quantity}).Error
		default:
			return err
		}
	})
	if err != nil {
		return nil, err
	}
	return GetCart(userID)
}

func UpdateItem(userID, itemID uint, input *models.UpdateCartItemDTO) (*models.CartResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	item, err := findOwnItem(userID, itemID)
	if err != nil {
		return nil, err
	}
	var m repository.ProductModel
	if err := config.DB.First(&m, item.ProductModelID).Error; err == nil && input.Quantity > m.Stock {
		return nil, fmt.Errorf("%w: stok tersisa %d", ErrValidation, m.Stock)
	}
	if err := config.DB.Model(item).Update("quantity", input.Quantity).Error; err != nil {
		return nil, fmt.Errorf("failed to update cart item: %w", err)
	}
	return GetCart(userID)
}

func RemoveItem(userID, itemID uint) (*models.CartResponseDTO, error) {
	item, err := findOwnItem(userID, itemID)
	if err != nil {
		return nil, err
	}
	if err := config.DB.Delete(item).Error; err != nil {
		return nil, fmt.Errorf("failed to remove cart item: %w", err)
	}
	return GetCart(userID)
}

func ClearCart(userID uint) error {
	sub := config.DB.Model(&repository.Cart{}).Select("id").Where("user_id = ?", userID)
	if err := config.DB.Where("cart_id IN (?)", sub).Delete(&repository.CartItem{}).Error; err != nil {
		return fmt.Errorf("failed to clear cart: %w", err)
	}
	return nil
}

// findOwnItem hanya menemukan item yang ada di keranjang milik userID.
func findOwnItem(userID, itemID uint) (*repository.CartItem, error) {
	var item repository.CartItem
	err := config.DB.
		Joins("JOIN carts ON carts.id = cart_items.cart_id AND carts.deleted_at IS NULL").
		Where("cart_items.id = ? AND carts.user_id = ?", itemID, userID).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("cart item %d: %w", itemID, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func getOrCreateCart(tx *gorm.DB, userID uint) (*repository.Cart, error) {
	var cart repository.Cart
	err := tx.Where("user_id = ?", userID).First(&cart).Error
	if err == nil {
		return &cart, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	cart = repository.Cart{UserID: userID}
	if err := tx.Create(&cart).Error; err != nil {
		return nil, fmt.Errorf("failed to create cart: %w", err)
	}
	return &cart, nil
}
