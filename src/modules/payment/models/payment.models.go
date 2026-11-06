package models

import "time"

// PaymentRequestDTO: nominal TIDAK dikirim klien; selalu diambil dari orders.total.
// (Gateway lama Midtrans, VA bank transfer. Xendit menyusul di M8.)
type PaymentRequestDTO struct {
	OrderID     uint   `json:"order_id" validate:"required"`
	PaymentType string `json:"payment_type" validate:"required,oneof=bank_transfer"`
	Bank        string `json:"bank" validate:"required,oneof=bca bni bri permata cimb"`
}

type VANumberDTO struct {
	Bank     string `json:"bank"`
	VANumber string `json:"va_number"`
}

type PaymentResponseDTO struct {
	ID                uint          `json:"id"`
	OrderID           uint          `json:"order_id"`
	Provider          string        `json:"provider"`
	ReferenceID       string        `json:"reference_id"`
	ProviderPaymentID *string       `json:"provider_payment_id,omitempty"`
	PaymentMethod     *string       `json:"payment_method,omitempty"`
	PaymentChannel    *string       `json:"payment_channel,omitempty"`
	Amount            int64         `json:"amount"`
	Currency          string        `json:"currency"`
	Status            string        `json:"status"`
	VANumbers         []VANumberDTO `json:"va_numbers,omitempty"`
	ExpiresAt         *time.Time    `json:"expires_at,omitempty"`
	PaidAt            *time.Time    `json:"paid_at,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
}
