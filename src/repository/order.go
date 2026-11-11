package repository

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------
// Keranjang — tidak menyimpan harga; harga selalu dihitung live dari Variant.
// ---------------------------------------------------------------------

type Cart struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null" json:"user_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`

	Items []CartItem `gorm:"foreignKey:CartID" json:"items,omitempty"`
}

type CartItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CartID    uint      `gorm:"not null" json:"cart_id"`
	VariantID uint      `gorm:"not null" json:"variant_id"`
	Variant   *Variant  `gorm:"foreignKey:VariantID" json:"variant,omitempty"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------
// Order — status VARCHAR + CHECK constraint (bukan tabel lookup).
// ---------------------------------------------------------------------

const (
	OrderPendingPayment = "PENDING_PAYMENT" // Belum Bayar (stok di-reserve)
	OrderPaid           = "PAID"            // Sudah bayar (stok terpotong)
	OrderProcessing     = "PROCESSING"      // Dikemas
	OrderShipped        = "SHIPPED"         // Dikirim
	OrderCompleted      = "COMPLETED"       // Selesai
	OrderCancelled      = "CANCELLED"       // Dibatalkan
	OrderExpired        = "EXPIRED"         // Dibatalkan otomatis (tidak dibayar)
	OrderRefunded       = "REFUNDED"        // Dana dikembalikan
)

// orderTransitions: transisi yang diizinkan (hanya maju).
var orderTransitions = map[string][]string{
	OrderPendingPayment: {OrderPaid, OrderCancelled, OrderExpired},
	OrderPaid:           {OrderProcessing, OrderRefunded},
	OrderProcessing:     {OrderShipped, OrderRefunded},
	OrderShipped:        {OrderCompleted, OrderRefunded},
}

// CanOrderTransition melaporkan apakah status order boleh berpindah dari → ke.
func CanOrderTransition(from, to string) bool {
	for _, s := range orderTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

type Order struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	OrderNo       string `gorm:"size:32;not null" json:"order_no"`
	UserID        uint   `gorm:"not null" json:"user_id"`
	User          *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Status        string `gorm:"size:20;not null;default:'PENDING_PAYMENT'" json:"status"`
	Subtotal      int64  `gorm:"not null" json:"subtotal"`
	ShippingFee   int64  `gorm:"not null" json:"shipping_fee"` // ongkir ditanggung toko → 0
	Total         int64  `gorm:"not null" json:"total"`
	TotalQuantity int    `gorm:"not null" json:"total_quantity"`

	// snapshot alamat
	RecipientName    string  `gorm:"size:255;not null" json:"recipient_name"`
	RecipientPhone   string  `gorm:"size:20;not null" json:"recipient_phone"`
	ShippingProvince string  `gorm:"size:100;not null" json:"shipping_province"`
	ShippingCity     string  `gorm:"size:100;not null" json:"shipping_city"`
	ShippingDistrict string  `gorm:"size:100;not null" json:"shipping_district"`
	ShippingPostal   string  `gorm:"size:10;not null" json:"shipping_postal"`
	ShippingAddress  string  `gorm:"not null" json:"shipping_address"`
	BuyerNote        *string `json:"buyer_note,omitempty"`

	PaidAt       *time.Time `json:"paid_at,omitempty"`
	ShippedAt    *time.Time `json:"shipped_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CancelledAt  *time.Time `json:"cancelled_at,omitempty"`
	CancelReason *string    `json:"cancel_reason,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`

	Items    []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	Payments []Payment   `gorm:"foreignKey:OrderID" json:"payments,omitempty"`
}

// OrderItem menyimpan SNAPSHOT saat checkout. Jangan pernah membaca harga
// order dari Variant — selalu dari UnitPrice/Subtotal di sini.
type OrderItem struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	OrderID       uint      `gorm:"not null" json:"order_id"`
	VariantID     *uint     `json:"variant_id,omitempty"` // referensi saja, bisa NULL
	ProductID     *uint     `json:"product_id,omitempty"`
	ProductName   string    `gorm:"size:255;not null" json:"product_name"`
	Grade         *string   `gorm:"size:20" json:"grade,omitempty"`
	ScentName     *string   `gorm:"size:100" json:"scent_name,omitempty"`
	SizeLabel     *string   `gorm:"size:20" json:"size_label,omitempty"`
	VolumeML      *int      `gorm:"column:volume_ml" json:"volume_ml,omitempty"`
	Concentration *string   `gorm:"size:10" json:"concentration,omitempty"`
	VariantLabel  *string   `gorm:"size:100" json:"variant_label,omitempty"`
	VariantName   *string   `gorm:"size:255" json:"variant_name,omitempty"`
	VariantCode   string    `gorm:"size:120;not null" json:"variant_code"`
	ImageURL      *string   `json:"image_url,omitempty"`
	UnitPrice     int64     `gorm:"not null" json:"unit_price"`                        // harga per unit saat checkout
	PricePerML    *int64    `gorm:"column:price_per_ml" json:"price_per_ml,omitempty"` // snapshot tarif (parfum)
	BottlePrice   *int64    `json:"bottle_price,omitempty"`                            // snapshot harga botol
	Quantity      int       `gorm:"not null" json:"quantity"`
	Subtotal      int64     `gorm:"not null" json:"subtotal"` // = UnitPrice * Quantity (dicek CHECK)
	WeightGram    int       `gorm:"not null" json:"weight_gram"`
	TrackStock    bool      `gorm:"not null" json:"track_stock"` // snapshot: stok di-reserve saat checkout
	CreatedAt     time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------
// Payment — Xendit Payment Sessions
// ---------------------------------------------------------------------

const (
	PaymentProviderXendit = "xendit"

	PaymentPending   = "PENDING"
	PaymentPaid      = "PAID"
	PaymentExpired   = "EXPIRED"
	PaymentCancelled = "CANCELLED"
	PaymentFailed    = "FAILED"
	PaymentRefunded  = "REFUNDED"
)

type Payment struct {
	ID                       uint           `gorm:"primaryKey" json:"id"`
	OrderID                  uint           `gorm:"not null" json:"order_id"`
	Provider                 string         `gorm:"size:20;not null;default:'xendit'" json:"provider"`
	ReferenceID              string         `gorm:"type:uuid;not null" json:"reference_id"`
	ProviderSessionID        *string        `gorm:"size:100" json:"provider_session_id,omitempty"`
	ProviderPaymentRequestID *string        `gorm:"size:100" json:"provider_payment_request_id,omitempty"`
	ProviderPaymentID        *string        `gorm:"size:100" json:"provider_payment_id,omitempty"`
	CheckoutURL              *string        `gorm:"column:checkout_url" json:"checkout_url,omitempty"`
	PaymentMethod            *string        `gorm:"size:50" json:"payment_method,omitempty"`
	PaymentChannel           *string        `gorm:"size:50" json:"payment_channel,omitempty"`
	Amount                   int64          `gorm:"not null" json:"amount"`
	Currency                 string         `gorm:"type:char(3);not null;default:'IDR'" json:"currency"`
	Status                   string         `gorm:"size:20;not null;default:'PENDING'" json:"status"`
	FailureReason            *string        `json:"failure_reason,omitempty"`
	ExpiresAt                *time.Time     `json:"expires_at,omitempty"`
	PaidAt                   *time.Time     `json:"paid_at,omitempty"`
	ProviderResponse         datatypes.JSON `gorm:"type:jsonb" json:"-"`
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`
	DeletedAt                gorm.DeletedAt `json:"-"`
}

// PaymentEvent: log webhook mentah, append-only (tanpa soft delete).
type PaymentEvent struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	Provider          string         `gorm:"size:20;not null" json:"provider"`
	EventType         string         `gorm:"size:100;not null" json:"event_type"`
	ProviderPaymentID string         `gorm:"size:100;not null" json:"provider_payment_id"`
	PaymentID         *uint          `json:"payment_id,omitempty"`
	Payload           datatypes.JSON `gorm:"type:jsonb;not null" json:"payload"`
	ReceivedAt        time.Time      `gorm:"not null;autoCreateTime" json:"received_at"`
	ProcessedAt       *time.Time     `json:"processed_at,omitempty"`
	ProcessError      *string        `json:"process_error,omitempty"`
}
