package models

import "backend/src/repository"

// ProductCard = 1 kartu di daftar produk.
type ProductCard struct {
	ID            uint                 `json:"id"`
	Kind          string               `json:"kind"`
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	Grade         *string              `json:"grade,omitempty"`
	InspiredBy    *string              `json:"inspired_by,omitempty"`
	NotePrimary   *string              `json:"note_primary,omitempty"`
	NoteSecondary *string              `json:"note_secondary,omitempty"`
	Category      *repository.Category `json:"category,omitempty"`
	ImageURL      *string              `json:"image_url,omitempty"`
	Scents        []string             `json:"scents,omitempty"` // nama wangian di grup ini
	PriceMin      int64                `json:"price_min"`
	PriceMax      int64                `json:"price_max"`
	InStock       bool                 `json:"in_stock"` // ada varian yang bisa dibeli sekarang
}

// VariantView = 1 pilihan yang bisa dibeli di halaman produk.
type VariantView struct {
	ID             uint    `json:"id"`
	ScentID        *uint   `json:"scent_id,omitempty"`
	SizeID         *uint   `json:"size_id,omitempty"`
	Concentration  *string `json:"concentration,omitempty"`
	Label          *string `json:"label,omitempty"`
	Name           string  `json:"name"` // "Dolly Girl, 30 ml, Murni"
	Code           string  `json:"code"`
	Price          int64   `json:"price"`
	CompareAtPrice *int64  `json:"compare_at_price,omitempty"`
	InStock        bool    `json:"in_stock"`
	Available      *int    `json:"available"` // stok tersedia; null = tidak dihitung (selalu tersedia)
}

// ProductDetail = halaman produk: pembeli pilih Varian (wangian) → Ukuran → Tipe.
type ProductDetail struct {
	ProductCard
	Description    string                    `json:"description"`
	Images         []repository.ProductImage `json:"images"`
	ScentOptions   []repository.Scent        `json:"scent_options"`
	SizeOptions    []repository.Size         `json:"size_options"`
	Concentrations []string                  `json:"concentrations"`
	Variants       []VariantView             `json:"variants"`
}
