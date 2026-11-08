package repository

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------
// Katalog: meniru toko & listing Shopee 1:1
//   Category                = "Kategori" di halaman toko Shopee (1 produk = 1 kategori)
//   Product                 = Item/listing Shopee (parfum = 1 produk, botol + isi jadi satu)
//   ProductVariation        = nama tingkat variasi ("Warna", "Ukuran", "Aroma")
//   ProductVariationOption  = pilihan di tingkat itu ("Murni", "25 ML")
//   ProductModel            = kombinasi pilihan = SKU (harga & stok)
// Tidak ada master botol/brand. Wangian ditautkan lewat ScentID.
// ---------------------------------------------------------------------

// ---------------------------------------------------------------------
// Wangian: struktur dari foto produk (1 foto = 1 grup, bullet = varian).
// Regular dan Exclusive adalah 2 jenis berbeda walaupun namanya sama
// (mis. "Moonlight" Escada ≠ "Moonlight" Exclusive), jadi kuncinya Code.
// ---------------------------------------------------------------------

const (
	GradeRegular   = "regular"
	GradeExclusive = "exclusive"
)

// ScentNotes = label karakter aroma yang dipakai di foto produk.
var ScentNotes = []string{"amber", "aromatic", "balsamic", "citrus", "creamy", "floral", "fresh", "fruity", "musk", "oud", "powdery", "sweet", "woody"}

type ScentGroup struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Code          string         `gorm:"size:100;not null" json:"code"`
	Grade         string         `gorm:"size:20;not null" json:"grade"`
	Name          string         `gorm:"size:100;not null" json:"name"`
	InspiredBy    *string        `gorm:"size:255" json:"inspired_by,omitempty"`
	NotePrimary   string         `gorm:"size:20;not null" json:"note_primary"`
	NoteSecondary string         `gorm:"size:20;not null" json:"note_secondary"`
	ImageURL      *string        `json:"image_url,omitempty"`
	SortOrder     int            `gorm:"not null" json:"sort_order"`
	IsActive      bool           `gorm:"not null" json:"is_active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-"`

	Scents []Scent `gorm:"foreignKey:GroupID" json:"scents,omitempty"`
}

type Scent struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	GroupID   uint           `gorm:"not null" json:"group_id"`
	Group     *ScentGroup    `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	Code      string         `gorm:"size:150;not null" json:"code"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	SortOrder int            `gorm:"not null" json:"sort_order"`
	IsActive  bool           `gorm:"not null" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

const (
	ProductStatusDraft    = "draft"
	ProductStatusActive   = "active"
	ProductStatusUnlisted = "unlisted"
)

type Category struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	ShopeeCategoryID *int64         `json:"shopee_category_id,omitempty"`
	Name             string         `gorm:"size:100;not null" json:"name"`
	Slug             string         `gorm:"size:100;not null" json:"slug"`
	SortOrder        int            `gorm:"not null" json:"sort_order"`
	IsActive         bool           `gorm:"not null" json:"is_active"` // tanpa tag default: false harus bisa disimpan
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-"`
}

type Product struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ShopeeItemID *int64         `json:"shopee_item_id,omitempty"`
	CategoryID   uint           `gorm:"not null" json:"category_id"`
	Category     *Category      `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	ScentID      *uint          `json:"scent_id,omitempty"` // listing 1 wangian
	Scent        *Scent         `gorm:"foreignKey:ScentID" json:"scent,omitempty"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	Slug         string         `gorm:"size:255;not null" json:"slug"`
	Description  string         `gorm:"not null" json:"description"`
	Status       string         `gorm:"size:20;not null;default:'draft'" json:"status"`
	ParentSKU    *string        `gorm:"column:parent_sku;size:100" json:"parent_sku,omitempty"`
	WeightGram   int            `gorm:"not null" json:"weight_gram"`
	SortOrder    int            `gorm:"not null" json:"sort_order"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-"`

	Images     []ProductImage     `gorm:"foreignKey:ProductID" json:"images,omitempty"`
	Variations []ProductVariation `gorm:"foreignKey:ProductID" json:"variations,omitempty"`
	Models     []ProductModel     `gorm:"foreignKey:ProductID" json:"models,omitempty"`
}

type ProductImage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProductID uint      `gorm:"not null" json:"product_id"`
	URL       string    `gorm:"column:url;not null" json:"url"`
	SortOrder int16     `gorm:"not null" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type ProductVariation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProductID uint      `gorm:"not null" json:"product_id"`
	TierIndex int16     `gorm:"not null" json:"tier_index"` // 0 atau 1
	Name      string    `gorm:"size:50;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Options []ProductVariationOption `gorm:"foreignKey:VariationID" json:"options,omitempty"`
}

type ProductVariationOption struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	VariationID uint      `gorm:"not null" json:"variation_id"`
	ProductID   uint      `gorm:"not null" json:"product_id"` // wajib diisi = product_id variasinya
	OptionIndex int16     `gorm:"not null" json:"option_index"`
	Label       string    `gorm:"size:100;not null" json:"label"`
	ImageURL    *string   `json:"image_url,omitempty"`
	ScentID     *uint     `json:"scent_id,omitempty"` // pilihan ini = wangian ini
	Scent       *Scent    `gorm:"foreignKey:ScentID" json:"scent,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProductModel struct {
	ID            uint                    `gorm:"primaryKey" json:"id"`
	ProductID     uint                    `gorm:"not null" json:"product_id"`
	Product       *Product                `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Option1ID     *uint                   `gorm:"column:option1_id" json:"option1_id,omitempty"`
	Option1       *ProductVariationOption `gorm:"foreignKey:Option1ID" json:"option1,omitempty"`
	Option2ID     *uint                   `gorm:"column:option2_id" json:"option2_id,omitempty"`
	Option2       *ProductVariationOption `gorm:"foreignKey:Option2ID" json:"option2,omitempty"`
	ShopeeModelID *int64                  `json:"shopee_model_id,omitempty"`
	SKU           *string                 `gorm:"column:sku;size:100" json:"sku,omitempty"`
	Price         int64                   `gorm:"not null" json:"price"`    // Rp
	OriginalPrice *int64                  `json:"original_price,omitempty"` // Rp, harga coret
	Stock         int                     `gorm:"not null" json:"stock"`
	WeightGram    *int                    `json:"weight_gram,omitempty"`
	IsActive      bool                    `gorm:"not null" json:"is_active"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	DeletedAt     gorm.DeletedAt          `json:"-"`
}

// VariationName menghasilkan teks variasi seperti di Shopee: "Murni, 25 ML".
// Option1/Option2 harus sudah di-Preload.
func (m ProductModel) VariationName() string {
	switch {
	case m.Option1 != nil && m.Option2 != nil:
		return m.Option1.Label + ", " + m.Option2.Label
	case m.Option1 != nil:
		return m.Option1.Label
	default:
		return ""
	}
}
