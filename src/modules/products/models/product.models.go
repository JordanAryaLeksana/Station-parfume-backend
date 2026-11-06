package models

import "backend/src/repository"

// ProductResponseDTO = listing seperti di Shopee: produk + variasi + model (SKU),
// ditambah ringkasan harga ("Rp45.000 - Rp65.000") dan total stok.
// Hanya model aktif dan pilihan yang dipakai model aktif yang ikut dikirim.
type ProductResponseDTO struct {
	repository.Product
	PriceMin   int64 `json:"price_min"`
	PriceMax   int64 `json:"price_max"`
	TotalStock int   `json:"total_stock"`
}

type BrandRequestDTO struct {
	Name        string  `json:"name" validate:"required,max=255"`
	Description *string `json:"description"`
	LogoURL     *string `json:"logo_url"`
}

type BottleRequestDTO struct {
	Name        string  `json:"name" validate:"required,max=255"`
	BottleType  string  `json:"bottle_type" validate:"required,max=30"`
	VolumeML    int     `json:"volume_ml" validate:"required,gt=0"`
	Description *string `json:"description"`
	ImageURL    *string `json:"image_url"`
	IsActive    *bool   `json:"is_active"` // kosong = true
}

// AromaUpdateDTO: semua field opsional; hanya yang diisi yang diubah.
// Nama aroma tidak bisa diubah di sini karena harus sama dengan label variasi
// di Shopee (ubah lewat seed/aromas.csv).
type AromaUpdateDTO struct {
	BrandID     *uint   `json:"brand_id"`
	Gender      *string `json:"gender" validate:"omitempty,oneof=men women unisex"`
	Grade       *string `json:"grade" validate:"omitempty,oneof=regular exclusive super_exclusive"`
	Description *string `json:"description"`
	TopNotes    *string `json:"top_notes"`
	MiddleNotes *string `json:"middle_notes"`
	BaseNotes   *string `json:"base_notes"`
	ImageURL    *string `json:"image_url"`
	IsActive    *bool   `json:"is_active"`
}
