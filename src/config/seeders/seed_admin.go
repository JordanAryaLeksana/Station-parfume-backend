package seeders

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"backend/src/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedAdmin membuat akun admin dari env ADMIN_NAME, ADMIN_EMAIL, ADMIN_PASSWORD.
// Password disimpan sebagai hash bcrypt. Kalau email sudah terdaftar, tidak
// ada yang diubah (aman dijalankan berulang kali). created=false berarti dilewati.
func SeedAdmin(db *gorm.DB) (created bool, err error) {
	name := strings.TrimSpace(os.Getenv("ADMIN_NAME"))
	email := repository.NormalizeEmail(os.Getenv("ADMIN_EMAIL"))
	password := os.Getenv("ADMIN_PASSWORD")
	if name == "" || email == "" || len(password) < 8 {
		return false, errors.New("ADMIN_NAME dan ADMIN_EMAIL wajib diisi, ADMIN_PASSWORD minimal 8 karakter")
	}

	var existing repository.User
	err = db.Where("lower(email) = ?", email).First(&existing).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("cek admin: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("hash password admin: %w", err)
	}
	h := string(hash)
	admin := repository.User{
		Name:         name,
		Email:        email,
		PasswordHash: &h,
		Provider:     repository.ProviderLocal,
		Role:         repository.RoleAdmin,
	}
	if err := db.Create(&admin).Error; err != nil {
		return false, fmt.Errorf("buat admin: %w", err)
	}
	return true, nil
}
