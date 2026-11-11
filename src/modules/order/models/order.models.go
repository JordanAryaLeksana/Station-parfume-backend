package models

// CheckoutRequestDTO: order dibuat dari isi keranjang user (harga dari DB, bukan dari klien).
// Alamat disalin ke order sebagai snapshot.
type CheckoutRequestDTO struct {
	RecipientName  string  `json:"recipient_name" validate:"required,max=255"`
	RecipientPhone string  `json:"recipient_phone" validate:"required,max=20"`
	Province       string  `json:"province" validate:"required,max=100"`
	City           string  `json:"city" validate:"required,max=100"`
	District       string  `json:"district" validate:"required,max=100"`
	PostalCode     string  `json:"postal_code" validate:"required,max=10"`
	Address        string  `json:"address" validate:"required"`
	BuyerNote      *string `json:"buyer_note"`
}

type CancelOrderDTO struct {
	Reason *string `json:"reason"`
}

// UpdateOrderStatusDTO (admin). Status: PAID, PROCESSING, SHIPPED, COMPLETED, CANCELLED, EXPIRED, REFUNDED.
type UpdateOrderStatusDTO struct {
	Status string  `json:"status" validate:"required"`
	Reason *string `json:"reason"`
}
