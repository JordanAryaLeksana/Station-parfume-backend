package repository

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"
)

type OrderStatus struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Status string `gorm:"not null" json:"status"`
}

// OrderStatusPending    = "pending"
// OrderStatusSuccess   = "success"
// OrderStatusFailed     = "failed"
// OrderStatusCancelled = "cancelled"

type PaymentStatus struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Status string `gorm:"not null" json:"status"`
}

// StatusPending    = "pending"
// 	StatusSettlement = "settlement"
// 	StatusCancel     = "cancel"
// 	StatusExpire     = "expire"

type FraudStatus struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Status string `gorm:"not null" json:"status"`
}

// FraudStatusAccept    = "accept"
// FraudStatusChallenge  = "challenge"
// FraudStatusReject     = "reject"

type Categories struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"unique;not null" json:"name"`
}
type Type struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"unique" json:"name"`
}

type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"not null;size:255" json:"name"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	Password  string         `json:"password"`
	Picture   string         `gorm:"" json:"picture"`
	Provider  string         `json:"provider"`
	Sub       *string        `gorm:"uniqueIndex" json:"sub"`
	Role      string         `gorm:"type:varchar(10);default:'user'" json:"role"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type Brand struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null;size:255" json:"name"`
	Description string    `gorm:"not null" json:"description"`
	Logo        string    `gorm:"not null" json:"logo"`
	Parfume     []Parfume `gorm:"foreignKey:BrandID" json:"parfume"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type Parfume struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	Name        string  `gorm:"not null;size:255" json:"name"`
	Description string  `gorm:"not null" json:"description"`
	PriceML     float64 `gorm:"not null" json:"price"`
	Image       string  `gorm:"not null" json:"image"`

	TypeID uint `gorm:"not null" json:"type_id"`
	Type   Type `json:"type"`

	CategoryID uint       `gorm:"not null" json:"category_id"`
	Category   Categories `json:"category"`

	BrandID uint  `gorm:"not null" json:"brand_id"`
	Brand   Brand `json:"brand"`

	Favorite  bool      `gorm:"not null;default:false" json:"favorite"`
	IsNew     bool      `gorm:"default:false" json:"is_new"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type Payment struct {
	ID      uint `gorm:"primaryKey" json:"id"`
	UserID  uint `gorm:"not null" json:"user_id"`
	OrderID uint `gorm:"not null" json:"order_id"`

	TransactionID string  `gorm:"uniqueIndex" json:"transaction_id"`
	OrderRef      string  `gorm:"not null;uniqueIndex" json:"order_ref"` 
	PaymentType   string  `gorm:"not null" json:"payment_type"`          
	Amount        float64 `gorm:"not null" json:"amount"`

	TransactionStatus string `gorm:"not null" json:"transaction_status"` // pending, settlement, cancel, expire
	FraudStatus       string `json:"fraud_status"`

	PaymentDetail datatypes.JSON `gorm:"type:json" json:"payment_detail"` // simpan VA/QR/action

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type Cart struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	UserID        uint       `gorm:"not null" json:"user_id"`
	User          User       `gorm:"foreignKey:UserID" json:"user"`
	Items         []CartItem `gorm:"foreignKey:CartID" json:"items"`
	TotalPrice    float64    `gorm:"not null" json:"total_price"`
	TotalQuantity int        `gorm:"not null" json:"total_quantity"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

type CartItem struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	CartID    uint    `gorm:"not null" json:"cart_id"`
	ParfumeID uint    `gorm:"not null" json:"parfume_id"`
	BottleID  uint    `gorm:"not null" json:"bottle_id"`
	Bottle    Bottle  `gorm:"foreignKey:BottleID" json:"bottle"`
	Parfume   Parfume `gorm:"foreignKey:ParfumeID" json:"parfume"`
	Quantity  int     `gorm:"not null" json:"quantity"`
}

type Order struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	UserID        uint        `gorm:"not null" json:"user_id"`
	User          User        `gorm:"foreignKey:UserID" json:"user"`
	Items         []OrderItem `gorm:"foreignKey:OrderID" json:"items"`
	TotalPrice    float64     `gorm:"not null" json:"total_price"`
	TotalQuantity int         `gorm:"not null" json:"total_quantity"`
	OrderStatus   OrderStatus `gorm:"foreignKey:OrderStatusID" json:"order_status"`
	OrderStatusID uint        `gorm:"not null" json:"order_status_id"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type OrderItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrderID   uint      `gorm:"not null" json:"order_id"`
	ParfumeID uint      `gorm:"not null" json:"parfume_id"`
	Parfume   Parfume   `gorm:"foreignKey:ParfumeID" json:"parfume"`
	BottleID  uint      `gorm:"not null" json:"bottle_id"`
	Bottle    Bottle    `gorm:"foreignKey:BottleID" json:"bottle"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	Price     float64   `gorm:"not null" json:"price"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type TypeBottle struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"unique;not null" json:"name"`
}
type Bottle struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"not null;size:255" json:"name"`
	Description  string     `gorm:"not null" json:"description"`
	Price        float64    `gorm:"not null" json:"price"`
	Image        string     `gorm:"not null" json:"image"`
	Size         float64    `gorm:"not null" json:"bottle_size"`
	IsNew        bool       `gorm:"not null" json:"is_new"`
	IsFavorite   bool       `gorm:"not null" json:"is_favorite"`
	TypeBottle   TypeBottle `gorm:"foreignKey:TypeBottleID" json:"type_bottle"`
	TypeBottleID uint       `gorm:"not null" json:"type_bottle_id"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
