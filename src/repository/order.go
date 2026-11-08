package repository

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------
// Keranjang — tidak menyimpan harga; harga selalu dibaca live dari ProductModel.
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
	ID             uint          `gorm:"primaryKey" json:"id"`
	CartID         uint          `gorm:"not null" json:"cart_id"`
	ProductModelID uint          `gorm:"not null" json:"product_model_id"`
	ProductModel   *ProductModel `gorm:"foreignKey:ProductModelID" json:"product_model,omitempty"`
	Quantity       int           `gorm:"not null" json:"quantity"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// ---------------------------------------------------------------------
// Order — status VARCHAR + CHECK constraint (bukan tabel lookup).
// ---------------------------------------------------------------------

const (
	OrderPendingPayment = "PENDING_PAYMENT" // Shopee: Belum Bayar
	OrderPaid           = "PAID"            // Shopee: Dikemas
	OrderProcessing     = "PROCESSING"      // Shopee: Dikemas
	OrderShipped        = "SHIPPED"         // Shopee: Dikirim
	OrderCompleted      = "COMPLETED"       // Shopee: Selesai
	OrderCancelled      = "CANCELLED"       // Shopee: Dibatalkan
	OrderExpired        = "EXPIRED"         // Shopee: Dibatalkan (otomatis)
	OrderRefunded       = "REFUNDED"        // Shopee: Pengembalian
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
	ShippingFee   int64  `gorm:"not null" json:"shipping_fee"`
	Discount      int64  `gorm:"not null" json:"discount"`
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
	Courier          *string `gorm:"size:50" json:"courier,omitempty"`
	TrackingNumber   *string `gorm:"size:100" json:"tracking_number,omitempty"`
	BuyerNote        *string `json:"buyer_note,omitempty"`

	PaidAt       *time.Time `json:"paid_at,omitempty"`
	ShippedAt    *time.Time `json:"shipped_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CancelledAt  *time.Time `json:"cancelled_at,omitempty"`
	CancelReason *string    `json:"cancel_reason,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`

	Items    []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	Payments []Payment   `gorm:"foreignKey:OrderID" json:"payments,omitempty"`
}

// OrderItem menyimpan SNAPSHOT saat checkout. Jangan pernah membaca harga
// order dari ProductModel — selalu dari UnitPrice/Subtotal di sini.
type OrderItem struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	OrderID           uint      `gorm:"not null" json:"order_id"`
	ProductModelID    *uint     `json:"product_model_id,omitempty"` // referensi saja, bisa NULL
	ProductID         *uint     `json:"product_id,omitempty"`
	ProductName       string    `gorm:"size:255;not null" json:"product_name"`
	VariationName     *string   `gorm:"size:255" json:"variation_name,omitempty"`
	Option1Label      *string   `gorm:"column:option1_label;size:100" json:"option1_label,omitempty"`
	Option2Label      *string   `gorm:"column:option2_label;size:100" json:"option2_label,omitempty"`
	SKU               *string   `gorm:"column:sku;size:100" json:"sku,omitempty"`
	ImageURL          *string   `json:"image_url,omitempty"`
	UnitPrice         int64     `gorm:"not null" json:"unit_price"`
	OriginalUnitPrice *int64    `json:"original_unit_price,omitempty"`
	Quantity          int       `gorm:"not null" json:"quantity"`
	Subtotal          int64     `gorm:"not null" json:"subtotal"` // = UnitPrice * Quantity (dicek CHECK)
	WeightGram        int       `gorm:"not null" json:"weight_gram"`
	CreatedAt         time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------
// Payment — Xendit Payment Sessions
// ---------------------------------------------------------------------

const (
	PaymentProviderXendit   = "xendit"
	PaymentProviderMidtrans = "midtrans"

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
