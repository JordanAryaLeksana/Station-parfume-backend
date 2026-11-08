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
