// Package repository berisi semua model GORM untuk Station Parfume:
// user.go, catalog.go, stock.go (riwayat stok), order.go.
//
// Sumber kebenaran schema adalah migrasi SQL di migrations/ (goose),
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
	// Pembeli.
	RoleUser = "user"
	// Staf toko (peran tetap).
	RoleOwner     = "owner"     // semua akses, termasuk kelola akun staf
	RoleAdmin     = "admin"     // produk, harga, stok, pesanan
	RoleWarehouse = "warehouse" // stok & pengiriman
	RoleCS        = "cs"        // lihat pesanan & pelanggan

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
}

// StaffRoles: semua peran staf toko.
var StaffRoles = []string{RoleOwner, RoleAdmin, RoleWarehouse, RoleCS}
