package services

import (
	"backend/src/config"
	"backend/src/modules/cart/models"
	"backend/src/repository"
	"fmt"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func CreateCart(userID uint, input *models.CartRequestDTO) (*models.CartResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}

	if userID != input.UserID {
		return nil, fmt.Errorf("user ID mismatch: %v != %v", userID, input.UserID)
	}

	var totalPrice float64 = 0
	var totalQuantity int = 0
	var cartItems []repository.CartItem

	// Hitung total dan siapkan cart items
	for _, item := range input.Items {
		var parfume repository.Parfume
		var bottle repository.Bottle

		if err := config.DB.First(&parfume, "id = ?", item.ParfumeID).Error; err != nil {
			return nil, fmt.Errorf("failed to find parfume: %v", err)
		}

		if err := config.DB.First(&bottle, "id = ?", item.BottleID).Error; err != nil {
			return nil, fmt.Errorf("failed to find bottle: %v", err)
		}

		quantity := item.Quantity
		price := parfume.PriceML + bottle.Price

		totalPrice += price * float64(quantity)
		totalQuantity += quantity

		cartItems = append(cartItems, repository.CartItem{
			ParfumeID: item.ParfumeID,
			BottleID:  item.BottleID,
			Quantity:  quantity,
		})
	}

	// Simpan Cart ke DB
	cart := repository.Cart{
		UserID:        userID,
		TotalPrice:    totalPrice,
		TotalQuantity: totalQuantity,
	}
	if err := config.DB.Create(&cart).Error; err != nil {
		return nil, fmt.Errorf("failed to create cart: %v", err)
	}

	// Simpan CartItems ke DB (dengan CartID dari cart yang baru dibuat)
	for i := range cartItems {
		cartItems[i].CartID = cart.ID
	}
	if err := config.DB.Create(&cartItems).Error; err != nil {
		return nil, fmt.Errorf("failed to create cart items: %v", err)
	}

	// Build response DTO
	var cartItemDTOs []models.CartItemDTO
	for _, item := range cartItems {
		cartItemDTOs = append(cartItemDTOs, models.CartItemDTO{
			ParfumeID: item.ParfumeID,
			BottleID:  item.BottleID,
			Quantity:  item.Quantity,
		})
	}

	return &models.CartResponseDTO{
		ID:            cart.ID,
		Items:         cartItemDTOs,
		TotalPrice:    cart.TotalPrice,
		TotalQuantity: cart.TotalQuantity,
	}, nil
}

func AddItemToCart(cartID uint, parfumeID uint, bottleID uint, quantity int) (*models.CartResponseDTO, error) {
	var cart repository.Cart
	if err := config.DB.Preload("Items").First(&cart, "id = ?", cartID).Error; err != nil {
		return nil, fmt.Errorf("cart not found: %v", err)
	}

	var parfume repository.Parfume
	var bottle repository.Bottle
	if err := config.DB.First(&parfume, "id = ?", parfumeID).Error; err != nil {
		return nil, fmt.Errorf("parfume not found: %v", parfumeID)
	}
	if err := config.DB.First(&bottle, "id = ?", bottleID).Error; err != nil {
		return nil, fmt.Errorf("bottle not found: %v", bottleID)
	}

	var item repository.CartItem
	err := config.DB.First(&item, "cart_id = ? AND parfume_id = ? AND bottle_id = ?", cartID, parfumeID, bottleID).Error
	if err == nil {
		item.Quantity += quantity
		if err := config.DB.Save(&item).Error; err != nil {
			return nil, err
		}
	} else {
		item = repository.CartItem{
			CartID:    cart.ID,
			ParfumeID: parfume.ID,
			BottleID:  bottle.ID,
			Quantity:  quantity,
		}
		if err := config.DB.Create(&item).Error; err != nil {
			return nil, err
		}
	}

	var items []repository.CartItem
	if err := config.DB.Preload("Parfume").Preload("Bottle").Find(&items, "cart_id = ?", cartID).Error; err != nil {
		return nil, err
	}

	var totalPrice float64
	var totalQty int
	for _, it := range items {
		totalQty += it.Quantity
		totalPrice += float64(it.Quantity) * (it.Parfume.PriceML + it.Bottle.Price)
	}

	cart.TotalQuantity = totalQty
	cart.TotalPrice = totalPrice
	if err := config.DB.Save(&cart).Error; err != nil {
		return nil, err
	}

	respItems := make([]models.CartItemDTO, 0)
	for _, it := range items {
		respItems = append(respItems, models.CartItemDTO{
			ID:        it.ID,
			CartID:    it.CartID,
			ParfumeID: it.ParfumeID,
			BottleID:  it.BottleID,
			Quantity:  it.Quantity,
		})
	}

	return &models.CartResponseDTO{
		ID:            cart.ID,
		Items:         respItems,
		TotalPrice:    cart.TotalPrice,
		TotalQuantity: cart.TotalQuantity,
	}, nil
}

func GetCartByUserID(userID uint) (*models.CartResponseDTO, error) {
	var cart repository.Cart
	if err := config.DB.Preload("Items").First(&cart, "user_id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("cart not found: %v", err)
	}

	var respItems []models.CartItemDTO
	for _, it := range cart.Items {
		respItems = append(respItems, models.CartItemDTO{
			ID:        it.ID,
			CartID:    it.CartID,
			ParfumeID: it.ParfumeID,
			BottleID:  it.BottleID,
			Quantity:  it.Quantity,
		})
	}

	return &models.CartResponseDTO{
		ID:            cart.ID,
		Items:         respItems,
		TotalPrice:    cart.TotalPrice,
		TotalQuantity: cart.TotalQuantity,
	}, nil
}

func ClearCart(cartID uint) error {
	var cart repository.Cart
	if err := config.DB.Preload("Items").First(&cart, "id = ?", cartID).Error; err != nil {
		return fmt.Errorf("cart not found: %v", err)
	}

	var cartItem repository.CartItem
	if err := config.DB.Where("cart_id = ?", cart.ID).Delete(&cartItem).Error; err != nil {
		return fmt.Errorf("failed to clear cart items: %v", err)
	}

	cart.TotalPrice = 0
	cart.TotalQuantity = 0
	if err := config.DB.Save(&cart).Error; err != nil {
		return fmt.Errorf("failed to reset cart: %v", err)
	}

	return nil
}
