package models

import "time"

// PaymentResponseDTO: 1 percobaan pembayaran Midtrans (Snap) untuk 1 order.
// Nominal selalu dari orders.total (klien tidak pernah mengirim harga).
type PaymentResponseDTO struct {
	ID          uint       `json:"id"`
	OrderID     uint       `json:"order_id"`
	Provider    string     `json:"provider"`
	ReferenceID string     `json:"reference_id"`
	Status      string     `json:"status"`       // PENDING, PAID, EXPIRED, CANCELLED, FAILED
	CheckoutURL *string    `json:"checkout_url"` // redirect pembeli ke halaman bayar Midtrans (Snap)
	SnapToken   *string    `json:"snap_token"`   // opsional: untuk popup snap.js (window.snap.pay)
	Amount      int64      `json:"amount"`
	Currency    string     `json:"currency"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
