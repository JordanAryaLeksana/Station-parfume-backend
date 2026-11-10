package repository

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------
// Stok per varian per lokasi. available = OnHand − Reserved.
// Reserved = tertahan order PENDING_PAYMENT; dipotong dari OnHand saat PAID,
// dilepas saat order CANCELLED/EXPIRED.
// ---------------------------------------------------------------------

const (
	MovementInitial    = "initial"
	MovementRestock    = "restock"
	MovementSale       = "sale"
	MovementReturn     = "return"
	MovementAdjustment = "adjustment"
	MovementOpname     = "opname"
)

type Location struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:30;not null" json:"code"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Phone       *string   `gorm:"size:20" json:"phone,omitempty"`
	Province    *string   `gorm:"size:100" json:"province,omitempty"`
	City        *string   `gorm:"size:100" json:"city,omitempty"`
	District    *string   `gorm:"size:100" json:"district,omitempty"`
	PostalCode  *string   `gorm:"size:10" json:"postal_code,omitempty"`
	AddressLine *string   `json:"address_line,omitempty"`
	AreaID      *string   `gorm:"size:64" json:"area_id,omitempty"`
	IsDefault   bool      `gorm:"not null" json:"is_default"`
	IsActive    bool      `gorm:"not null" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type InventoryLevel struct {
	VariantID  uint      `gorm:"primaryKey" json:"variant_id"`
	LocationID uint      `gorm:"primaryKey" json:"location_id"`
	OnHand     int       `gorm:"not null" json:"on_hand"`
	Reserved   int       `gorm:"not null" json:"reserved"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (l InventoryLevel) Available() int { return l.OnHand - l.Reserved }

type StockMovement struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	VariantID   uint      `gorm:"not null" json:"variant_id"`
	LocationID  uint      `gorm:"not null" json:"location_id"`
	Delta       int       `gorm:"not null" json:"delta"`
	OnHandAfter int       `gorm:"not null" json:"on_hand_after"`
	Reason      string    `gorm:"size:20;not null" json:"reason"`
	OrderID     *uint     `json:"order_id,omitempty"`
	UserID      *uint     `json:"user_id,omitempty"`
	Note        *string   `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------
// Harga reseller
// ---------------------------------------------------------------------

type ResellerLevel struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Name            string         `gorm:"size:50;not null" json:"name"`
	DiscountPercent int16          `gorm:"not null" json:"discount_percent"`
	IsActive        bool           `gorm:"not null" json:"is_active"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-"`
}

// VariantResellerPrice: harga khusus reseller untuk 1 varian (mengalahkan diskon %).
type VariantResellerPrice struct {
	VariantID       uint      `gorm:"primaryKey" json:"variant_id"`
	ResellerLevelID uint      `gorm:"primaryKey" json:"reseller_level_id"`
	Price           int64     `gorm:"not null" json:"price"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------
// Voucher & promo (tabel disiapkan; fitur dibangun nanti)
// ---------------------------------------------------------------------

const (
	VoucherPercent      = "percent"
	VoucherFixed        = "fixed"
	VoucherFreeShipping = "free_shipping"
)

type Voucher struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Code            string         `gorm:"size:40;not null" json:"code"`
	Name            string         `gorm:"size:100;not null" json:"name"`
	DiscountType    string         `gorm:"size:20;not null" json:"discount_type"`
	DiscountValue   int64          `gorm:"not null" json:"discount_value"`
	MaxDiscount     *int64         `json:"max_discount,omitempty"`
	MinSubtotal     int64          `gorm:"not null" json:"min_subtotal"`
	Quota           *int           `json:"quota,omitempty"`
	PerUserLimit    *int           `json:"per_user_limit,omitempty"`
	UsedCount       int            `gorm:"not null" json:"used_count"`
	ResellerAllowed bool           `gorm:"not null" json:"reseller_allowed"`
	StartsAt        time.Time      `gorm:"not null" json:"starts_at"`
	EndsAt          time.Time      `gorm:"not null" json:"ends_at"`
	IsActive        bool           `gorm:"not null" json:"is_active"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-"`
}

type VoucherRedemption struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VoucherID uint      `gorm:"not null" json:"voucher_id"`
	OrderID   uint      `gorm:"not null" json:"order_id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	Amount    int64     `gorm:"not null" json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

type Promotion struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	StartsAt  time.Time      `gorm:"not null" json:"starts_at"`
	EndsAt    time.Time      `gorm:"not null" json:"ends_at"`
	IsActive  bool           `gorm:"not null" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`

	Items []PromotionItem `gorm:"foreignKey:PromotionID" json:"items,omitempty"`
}

type PromotionItem struct {
	PromotionID uint  `gorm:"primaryKey" json:"promotion_id"`
	VariantID   uint  `gorm:"primaryKey" json:"variant_id"`
	SalePrice   int64 `gorm:"not null" json:"sale_price"`
	Quota       *int  `json:"quota,omitempty"`
	Sold        int   `gorm:"not null" json:"sold"`
}

// ---------------------------------------------------------------------
// Pengiriman
// ---------------------------------------------------------------------

const (
	ShipmentProviderManual = "manual"
	ShipmentPending        = "pending"
	ShipmentInTransit      = "in_transit"
	ShipmentDelivered      = "delivered"
)

type Shipment struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	OrderID         uint       `gorm:"not null" json:"order_id"`
	Provider        string     `gorm:"size:20;not null;default:'manual'" json:"provider"`
	CourierCode     *string    `gorm:"size:30" json:"courier_code,omitempty"`
	CourierService  *string    `gorm:"size:50" json:"courier_service,omitempty"`
	TrackingNumber  *string    `gorm:"size:100" json:"tracking_number,omitempty"`
	Cost            int64      `gorm:"not null" json:"cost"`
	WeightGram      int        `gorm:"not null" json:"weight_gram"`
	Status          string     `gorm:"size:20;not null;default:'pending'" json:"status"`
	ProviderOrderID *string    `gorm:"size:100" json:"provider_order_id,omitempty"`
	ShippedAt       *time.Time `json:"shipped_at,omitempty"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ---------------------------------------------------------------------
// Audit log
// ---------------------------------------------------------------------

type AuditLog struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    *uint          `json:"user_id,omitempty"`
	Action    string         `gorm:"size:50;not null" json:"action"`
	Entity    string         `gorm:"size:50;not null" json:"entity"`
	EntityID  *uint          `json:"entity_id,omitempty"`
	Changes   datatypes.JSON `gorm:"type:jsonb" json:"changes,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}
