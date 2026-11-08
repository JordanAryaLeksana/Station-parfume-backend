// Package repository berisi semua model GORM (schema v2, Shopee-aligned)
// untuk Station Parfume: user.go, catalog.go, order.go.
//
// Sumber kebenaran schema adalah docs/schema/schema.sql (migrasi SQL),
// BUKAN AutoMigrate: partial unique index, CHECK constraint, dan FK komposit
// tidak bisa diekspresikan lewat tag GORM. Tag di sini hanya untuk mapping.
//
// Aturan uang: semua harga/nominal bertipe int64 = rupiah utuh (BIGINT di DB).
package repository

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	ProviderLocal  = "local"
	ProviderGoogle = "google"
)

// NormalizeEmail: email disimpan & dicari dalam huruf kecil, sesuai unique
// index users_email_uq ON lower(email). Selalu cari dengan
// Where("lower(email) = ?", NormalizeEmail(x)).
func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	Email        string         `gorm:"size:255;not null" json:"email"`
	Phone        *string        `gorm:"size:20" json:"phone,omitempty"`
	PasswordHash *string        `gorm:"size:255" json:"-"`
	PictureURL   *string        `json:"picture_url,omitempty"`
	Provider     string         `gorm:"size:20;not null;default:'local'" json:"provider"`
	ProviderSub  *string        `gorm:"size:255" json:"-"`
	Role         string         `gorm:"size:10;not null;default:'user'" json:"role"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	Addresses []UserAddress `gorm:"foreignKey:UserID" json:"addresses,omitempty"`
}

type UserAddress struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	Label         *string        `gorm:"size:30" json:"label,omitempty"`
	RecipientName string         `gorm:"size:255;not null" json:"recipient_name"`
	Phone         string         `gorm:"size:20;not null" json:"phone"`
	Province      string         `gorm:"size:100;not null" json:"province"`
	City          string         `gorm:"size:100;not null" json:"city"`
	District      string         `gorm:"size:100;not null" json:"district"`
	PostalCode    string         `gorm:"size:10;not null" json:"postal_code"`
	AddressLine   string         `gorm:"not null" json:"address_line"`
	IsDefault     bool           `gorm:"not null" json:"is_default"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-"`
}
