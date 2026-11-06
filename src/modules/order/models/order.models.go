package models

import "time"

type OrderRequestDTO struct {
	UserID        uint           `json:"user_id" validate:"required"`
	PaymentID     uint           `json:"payment_id" validate:"required"`
	Items         []OrderItemDTO `json:"items" validate:"required,dive"`
	OrderStatusID uint           `json:"order_status_id" validate:"required"`
	TotalPrice    float64        `json:"total_price" validate:"required"`
	TotalQuantity int            `json:"total_quantity" validate:"required"`
}

type OrderItemDTO struct {
	ID        uint `json:"id"`
	OrderID   uint `json:"order_id" validate:"required"`
	ParfumeID uint `json:"parfume_id" validate:"required"`
	BottleID  uint `json:"bottle_id" validate:"required"`
	Quantity  int  `json:"quantity" validate:"required,gte=1"`
	Price    float64 `json:"price" validate:"required,gte=0"`
}

type OrderResponseDTO struct {
	ID            uint           `json:"id"`
	UserID       uint           `json:"user_id"`
	PaymentID     uint           `json:"payment_id"`
	OrderStatusID uint           `json:"order_status_id"`
	OrderStatus   OrderStatusDTO `json:"order_status"`
	Items         []OrderItemDTO `json:"items"`
	TotalPrice    float64        `json:"total_price"`
	TotalQuantity int            `json:"total_quantity"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}



type PaymentStatusDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type OrderStatusDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}