package models

type CartRequestDTO struct {
    Items         []CartItemDTO `json:"items" validate:"required,dive"`
    UserID        uint        `json:"user_id" validate:"required"`
    TotalPrice    float64       `json:"total_price" validate:"gte=0"`
    TotalQuantity int           `json:"total_quantity" validate:"gte=1"`
}

type CartItemDTO struct {
	ID        uint `json:"id"`
	CartID    uint `json:"cart_id" validate:"required"`
	ParfumeID uint `json:"parfume_id" validate:"required"`
	BottleID  uint `json:"bottle_id" validate:"required"`
	Quantity  int    `json:"quantity" validate:"required,gte=1"`
}

type CartResponseDTO struct {
	ID           uint        `json:"id"`
	Items        []CartItemDTO `json:"items"`
	TotalPrice   float64       `json:"total_price"`
	TotalQuantity int          `json:"total_quantity"`
}