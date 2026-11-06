package repository

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------
// Master data (internal): brand, aroma, botol
// ---------------------------------------------------------------------

const (
	GenderMen    = "men"
	GenderWomen  = "women"
	GenderUnisex = "unisex"

	GradeRegular        = "regular"
	GradeExclusive      = "exclusive"
	GradeSuperExclusive = "super_exclusive"
)

type Brand struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	Slug        string         `gorm:"size:255;not null" json:"slug"`
	Description *string        `json:"description,omitempty"`
	LogoURL     *string        `json:"logo_url,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

type Aroma struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	BrandID     *uint          `gorm:"index" json:"brand_id,omitempty"`
	Brand       *Brand         `gorm:"foreignKey:BrandID" json:"brand,omitempty"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	Slug        string         `gorm:"size:255;not null" json:"slug"`
	Description *string        `json:"description,omitempty"`
	Gender      string         `gorm:"size:10;not null;default:'unisex'" json:"gender"`
	Grade       string         `gorm:"size:20;not null;default:'regular'" json:"grade"`
	TopNotes    *string        `json:"top_notes,omitempty"`
	MiddleNotes *string        `json:"middle_notes,omitempty"`
	BaseNotes   *string        `json:"base_notes,omitempty"`
	ImageURL    *string        `json:"image_url,omitempty"`
	IsActive    bool           `gorm:"not null" json:"is_active"` // tanpa tag default: false harus bisa disimpan
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

type Bottle struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	Slug        string         `gorm:"size:255;not null" json:"slug"`
	BottleType  string         `gorm:"size:30;not null" json:"bottle_type"`
	VolumeML    int            `gorm:"column:volume_ml;not null" json:"volume_ml"`
	Description *string        `json:"description,omitempty"`
	ImageURL    *string        `json:"image_url,omitempty"`
	IsActive    bool           `gorm:"not null" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

// ---------------------------------------------------------------------
// Katalog: meniru listing Shopee 1:1
//   Product                 = Item/listing Shopee
//   ProductVariation        = nama tingkat variasi ("Aroma", "Ukuran")
//   ProductVariationOption  = pilihan di tingkat itu
//   ProductModel            = kombinasi pilihan = SKU (harga & stok)
// ---------------------------------------------------------------------

const (
	ProductTypePerfume     = "perfume"
	ProductTypeEmptyBottle = "empty_bottle"
	ProductTypeOther       = "other"

	ProductStatusDraft    = "draft"
	ProductStatusActive   = "active"
	ProductStatusUnlisted = "unlisted"
)

type ShopCategory struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	Slug      string         `gorm:"size:100;not null" json:"slug"`
	SortOrder int            `gorm:"not null" json:"sort_order"`
	IsActive  bool           `gorm:"not null" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

type Product struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ShopeeItemID *int64         `json:"shopee_item_id,omitempty"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	Slug         string         `gorm:"size:255;not null" json:"slug"`
	Description  string         `gorm:"not null" json:"description"`
	ProductType  string         `gorm:"size:20;not null" json:"product_type"`
	Status       string         `gorm:"size:20;not null;default:'draft'" json:"status"`
	ParentSKU    *string        `gorm:"column:parent_sku;size:100" json:"parent_sku,omitempty"`
	WeightGram   int            `gorm:"not null" json:"weight_gram"`
	SortOrder    int            `gorm:"not null" json:"sort_order"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-"`

	Images         []ProductImage     `gorm:"foreignKey:ProductID" json:"images,omitempty"`
	Variations     []ProductVariation `gorm:"foreignKey:ProductID" json:"variations,omitempty"`
	Models         []ProductModel     `gorm:"foreignKey:ProductID" json:"models,omitempty"`
	ShopCategories []ShopCategory     `gorm:"many2many:product_shop_categories;joinForeignKey:ProductID;joinReferences:ShopCategoryID" json:"shop_categories,omitempty"`
}

// ProductShopCategory adalah tabel join etalase ↔ produk.
type ProductShopCategory struct {
	ProductID      uint `gorm:"primaryKey"`
	ShopCategoryID uint `gorm:"primaryKey"`
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
	AromaID     *uint     `json:"aroma_id,omitempty"`
	Aroma       *Aroma    `gorm:"foreignKey:AromaID" json:"aroma,omitempty"`
	BottleID    *uint     `json:"bottle_id,omitempty"`
	Bottle      *Bottle   `gorm:"foreignKey:BottleID" json:"bottle,omitempty"`
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

// VariationName menghasilkan teks variasi seperti di Shopee: "Baccarat Rouge, 30ml Spray".
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
