package repository

import (
	"gorm.io/gorm"
	"time"
)


//orderStatus
const (
	OrderStatusPending    = "pending"
	OrderStatusSuccess   = "success"
	OrderStatusFailed     = "failed"
	OrderStatusCancelled = "cancelled"
)
var AllowedOrderStatus = map[string]bool{
	OrderStatusPending:    true,
	OrderStatusSuccess:   true,
	OrderStatusFailed:     true,
	OrderStatusCancelled: true,
}

const (
	Mens   = "mens"
	Womens = "womens"
	Unisex = "unisex"
)

var AllowedCategory = map[string]bool{
	Mens:   true,
	Womens: true,
	Unisex: true,
}

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

var AllowedRoles = map[string]bool{
	RoleUser:  true,
	RoleAdmin: true,
}


const (
	StatusPending    = "pending"
	StatusSettlement = "settlement"
	StatusCancel     = "cancel"
	StatusExpire     = "expire"
)
var AllowedPaymentStatus = map[string]bool{
	StatusPending:    true,
	StatusSettlement: true,
	StatusCancel:     true,
	StatusExpire:     true,
}

//fraud status
const (
	FraudStatusAccept    = "accept"
	FraudStatusChallenge = "challenge"
	FraudStatusReject   = "reject"
)
var AllowedFraudStatus = map[string]bool{
	FraudStatusAccept:    true,
	FraudStatusChallenge: true,
	FraudStatusReject:   true,
}

type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"not null;size:255" json:"name"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	Password  string         `json:"password"`
	Picture   string         `gorm:"" json:"picture"`
	Provider  string         `json:"provider"`
	Sub       string         `gorm:"uniqueIndex" json:"sub"`
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
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type Parfume struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null;size:255" json:"name"`
	Description string    `gorm:"not null" json:"description"`
	Price       float64   `gorm:"not null" json:"price"`
	Image       string    `gorm:"not null" json:"image"`
	Category    string    `gorm:"not null;type:varchar(10)" json:"category"`
	BrandID     uint      `gorm:"not null" json:"brand_id"`
	Brand       Brand     `gorm:"foreignKey:BrandID" json:"brand"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type Payment struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"not null" json:"user_id"`
	User   User `gorm:"foreignKey:UserID" json:"user"`
	OrderID       string  `gorm:"uniqueIndex;not null" json:"order_id"`             // Ex: "ORDER-101"
	TransactionID string  `gorm:"uniqueIndex" json:"transaction_id"`                // Ex: "abcd1234"
	PaymentMethod string  `gorm:"not null" json:"payment_method"`                   // Ex: "gopay", "bank_transfer"
	Amount        float64 `gorm:"not null" json:"amount"`                           // Gross amount
	Status        string  `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending, settlement, cancel, expire
	FraudStatus   string  `gorm:"type:varchar(20)" json:"fraud_status"`             // accept, challenge, etc.
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type Admin struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user"`
	Role      string    `gorm:"type:varchar(10);default:'admin'" json:"role"`
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
	Status        string      `gorm:"type:varchar(20);default:'pending'" json:"status"`
	PaymentID     uint        `gorm:"not null" json:"payment_id"`
	Payment       Payment     `gorm:"foreignKey:PaymentID" json:"payment"`
	CreatedAt     time.Time   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
}

type OrderItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrderID   uint      `gorm:"not null" json:"order_id"`
	ParfumeID uint      `gorm:"not null" json:"parfume_id"`
	Parfume   Parfume   `gorm:"foreignKey:ParfumeID" json:"parfume"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	Price     float64   `gorm:"not null" json:"price"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
