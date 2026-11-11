package services

import (
	"backend/src/catalog"
	"backend/src/config"
	"backend/src/modules/cart/models"
	"backend/src/repository"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var validate = validator.New()

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

// PreloadCartItem memuat semua yang dibutuhkan untuk menampilkan / checkout item keranjang.
func PreloadCartItem(db *gorm.DB) *gorm.DB {
	return catalog.PreloadVariant(db, "Variant")
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
		dto := models.CartItemResponseDTO{ID: it.ID, VariantID: it.VariantID, Quantity: it.Quantity}
		if v := it.Variant; v != nil {
			dto.ProductID = v.ProductID
			dto.VariantName = v.DisplayName()
			dto.VariantCode = v.Code
			dto.UnitPrice = catalog.UnitPrice(v)
			dto.CompareAtPrice = v.CompareAtPrice
			dto.Subtotal = dto.UnitPrice * int64(it.Quantity)
			dto.Stock = catalog.StockOf(v)
			if p := v.Product; p != nil {
				dto.ProductName, dto.ProductSlug, dto.Grade = p.Name, p.Slug, p.Grade
				if len(p.Images) > 0 {
					dto.ImageURL = &p.Images[0].URL
				}
			}
			dto.Available = catalog.Purchasable(v) && catalog.HasStock(v, it.Quantity)
		}
		if dto.Available {
			resp.TotalQuantity += it.Quantity
			resp.TotalPrice += dto.Subtotal
		}
		resp.Items = append(resp.Items, dto)
	}
	return resp, nil
}

// AddItem menambah varian ke keranjang user (keranjang dibuat otomatis).
// Kalau varian sudah ada di keranjang, jumlahnya ditambahkan.
func AddItem(userID uint, input *models.AddCartItemDTO) (*models.CartResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var v repository.Variant
		if err := catalog.PreloadVariant(tx, "").First(&v, input.VariantID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("variant %d: %w", input.VariantID, ErrNotFound)
			}
			return err
		}
		if !catalog.Purchasable(&v) {
			return fmt.Errorf("%w: produk tidak tersedia", ErrValidation)
		}
		cart, err := getOrCreateCart(tx, userID)
		if err != nil {
			return err
		}
		qty := input.Quantity
		var item repository.CartItem
		err = tx.Where("cart_id = ? AND variant_id = ?", cart.ID, v.ID).First(&item).Error
		if err == nil {
			qty += item.Quantity
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := checkStock(&v, qty); err != nil {
			return err
		}
		// Upsert: aman walau 2 request bersamaan menambah varian yang sama.
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "cart_id"}, {Name: "variant_id"}},
			DoUpdates: clause.Assignments(map[string]any{"quantity": qty, "updated_at": gorm.Expr("now()")}),
		}).Create(&repository.CartItem{CartID: cart.ID, VariantID: v.ID, Quantity: qty}).Error
	})
	if err != nil {
		return nil, err
	}
	return GetCart(userID)
}

// checkStock: v.Product harus di-Preload (untuk track_stock). Stok final
// dicek lagi (dengan lock) saat checkout.
func checkStock(v *repository.Variant, qty int) error {
	if s := catalog.StockOf(v); s != nil && qty > *s {
		return fmt.Errorf("%w: stok tersisa %d", ErrValidation, *s)
	}
	return nil
}

func UpdateItem(userID, itemID uint, input *models.UpdateCartItemDTO) (*models.CartResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	item, err := findOwnItem(userID, itemID)
	if err != nil {
		return nil, err
	}
	var v repository.Variant
	if err := config.DB.Preload("Product").First(&v, item.VariantID).Error; err != nil {
		return nil, err
	}
	if err := checkStock(&v, input.Quantity); err != nil {
		return nil, err
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

// getOrCreateCart aman untuk request bersamaan: INSERT ... ON CONFLICT DO NOTHING
// pada unique index carts_user_uq, lalu baca ulang.
func getOrCreateCart(tx *gorm.DB, userID uint) (*repository.Cart, error) {
	if err := tx.Exec(`INSERT INTO carts (user_id) VALUES (?) ON CONFLICT (user_id) WHERE deleted_at IS NULL DO NOTHING`, userID).Error; err != nil {
		return nil, fmt.Errorf("failed to create cart: %w", err)
	}
	var cart repository.Cart
	if err := tx.Where("user_id = ?", userID).First(&cart).Error; err != nil {
		return nil, err
	}
	return &cart, nil
}
