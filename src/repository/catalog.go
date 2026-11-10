package repository

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------
// Katalog (schema v3, lepas dari Shopee)
//   Product  = 1 halaman. Perfume = 1 grup foto (mis. "Annasui"); General = produk biasa.
//   Scent    = varian wangian di produk parfum ("Dolly Girl").
//   Size     = ukuran botol (20/30/50 ml), bisa ditambah admin.
//   Variant  = yang dibeli: wangian × ukuran × murni/campur (parfum) atau label (general).
// Harga parfum = tarif per ml (Scent) × volume (Size) + harga botol (Size);
// Variant.Price opsional sebagai harga khusus. Lihat catalog.ListPrice.
// Stok di InventoryLevel (per lokasi).
// ---------------------------------------------------------------------

const (
	ProductKindPerfume = "perfume"
	ProductKindGeneral = "general"

	ProductStatusDraft    = "draft"
	ProductStatusActive   = "active"
	ProductStatusArchived = "archived"

	GradeRegular   = "regular"
	GradeExclusive = "exclusive"

	ConcentrationMurni  = "murni"
	ConcentrationCampur = "campur"
)

// Concentrations = tipe parfum yang dijual (tetap: Murni & Campur).
var Concentrations = []string{ConcentrationMurni, ConcentrationCampur}

// ScentNotes = label karakter aroma yang dipakai di foto produk.
var ScentNotes = []string{"amber", "aromatic", "balsamic", "citrus", "creamy", "floral", "fresh", "fruity", "musk", "oud", "powdery", "sweet", "woody"}

type Category struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ParentID  *uint          `json:"parent_id,omitempty"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	Slug      string         `gorm:"size:100;not null" json:"slug"`
	SortOrder int            `gorm:"not null" json:"sort_order"`
	IsActive  bool           `gorm:"not null" json:"is_active"` // tanpa tag default: false harus bisa disimpan
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

type Product struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CategoryID    uint           `gorm:"not null" json:"category_id"`
	Category      *Category      `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Kind          string         `gorm:"size:10;not null" json:"kind"`
	Code          string         `gorm:"size:60;not null" json:"code"`
	Name          string         `gorm:"size:255;not null" json:"name"`
	Slug          string         `gorm:"size:255;not null" json:"slug"`
	Description   string         `gorm:"not null" json:"description"`
	Grade         *string        `gorm:"size:20" json:"grade,omitempty"`
	InspiredBy    *string        `gorm:"size:255" json:"inspired_by,omitempty"`
	NotePrimary   *string        `gorm:"size:20" json:"note_primary,omitempty"`
	NoteSecondary *string        `gorm:"size:20" json:"note_secondary,omitempty"`
	WeightGram    int            `gorm:"not null" json:"weight_gram"`
	TrackStock    bool           `gorm:"not null" json:"track_stock"` // false = selalu tersedia (parfum diracik sendiri)
	Status        string         `gorm:"size:20;not null;default:'draft'" json:"status"`
	SortOrder     int            `gorm:"not null" json:"sort_order"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-"`

	Images []ProductImage `gorm:"foreignKey:ProductID" json:"images,omitempty"`
	// Sizes: ukuran khusus produk ini (product_sizes). Kosong = pakai ukuran default.
	Sizes    []Size    `gorm:"many2many:product_sizes;joinForeignKey:ProductID;joinReferences:SizeID" json:"sizes,omitempty"`
	Scents   []Scent   `gorm:"foreignKey:ProductID" json:"scents,omitempty"`
	Variants []Variant `gorm:"foreignKey:ProductID" json:"variants,omitempty"`
}

type Scent struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ProductID uint   `gorm:"not null" json:"product_id"`
	Code      string `gorm:"size:100;not null" json:"code"`
	Name      string `gorm:"size:100;not null" json:"name"`
	// Tarif per ml: murni = "bibit", campur = "parfum". NULL = belum ada tarif (tidak dijual).
	PricePerMLMurni  *int64         `gorm:"column:price_per_ml_murni" json:"price_per_ml_murni"`
	PricePerMLCampur *int64         `gorm:"column:price_per_ml_campur" json:"price_per_ml_campur"`
	SortOrder        int            `gorm:"not null" json:"sort_order"`
	IsActive         bool           `gorm:"not null" json:"is_active"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-"`
}

// Rate: tarif per ml untuk tipe murni/campur (nil = belum ada tarif).
func (s Scent) Rate(concentration string) *int64 {
	if concentration == ConcentrationMurni {
		return s.PricePerMLMurni
	}
	return s.PricePerMLCampur
}

// ProductSize: baris join product_sizes (ukuran khusus 1 produk).
type ProductSize struct {
	ProductID uint `gorm:"primaryKey"`
	SizeID    uint `gorm:"primaryKey"`
}

type ProductImage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProductID uint      `gorm:"not null" json:"product_id"`
	ScentID   *uint     `json:"scent_id,omitempty"`
	URL       string    `gorm:"column:url;not null" json:"url"`
	SortOrder int16     `gorm:"not null" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// Size: 1 ukuran = 1 jenis botol, jadi harga botol ada di sini.
type Size struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Label       string    `gorm:"size:20;not null" json:"label"`
	VolumeML    int       `gorm:"column:volume_ml;not null" json:"volume_ml"`
	BottlePrice int64     `gorm:"not null" json:"bottle_price"`
	WeightGram  int       `gorm:"not null" json:"weight_gram"`
	IsDefault   bool      `gorm:"not null" json:"is_default"` // dijual untuk semua parfum
	SortOrder   int       `gorm:"not null" json:"sort_order"`
	IsActive    bool      `gorm:"not null" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Variant struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ProductID      uint           `gorm:"not null" json:"product_id"`
	Product        *Product       `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	ScentID        *uint          `json:"scent_id,omitempty"`
	Scent          *Scent         `gorm:"foreignKey:ScentID" json:"scent,omitempty"`
	SizeID         *uint          `json:"size_id,omitempty"`
	Size           *Size          `gorm:"foreignKey:SizeID" json:"size,omitempty"`
	Concentration  *string        `gorm:"size:10" json:"concentration,omitempty"`
	Label          *string        `gorm:"size:100" json:"label,omitempty"`
	Code           string         `gorm:"size:120;not null" json:"code"`
	Price          *int64         `json:"price"`                      // parfum: harga khusus (NULL = rumus); general: harga jual
	CompareAtPrice *int64         `json:"compare_at_price,omitempty"` // harga coret
	WeightGram     *int           `json:"weight_gram,omitempty"`
	IsActive       bool           `gorm:"not null" json:"is_active"`
	SortOrder      int            `gorm:"not null" json:"sort_order"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-"`
}

// DisplayName: teks varian untuk keranjang/order, mis. "Dolly Girl, 30 ml, Murni".
// Scent dan Size harus sudah di-Preload.
func (v Variant) DisplayName() string {
	var parts []string
	if v.Scent != nil {
		parts = append(parts, v.Scent.Name)
	}
	if v.Size != nil {
		parts = append(parts, v.Size.Label)
	}
	if v.Concentration != nil {
		parts = append(parts, ConcentrationLabel(*v.Concentration))
	}
	if v.Label != nil {
		parts = append(parts, *v.Label)
	}
	return strings.Join(parts, ", ")
}

// ShippingWeight: berat kirim per unit (varian → ukuran → produk).
func (v Variant) ShippingWeight() int {
	switch {
	case v.WeightGram != nil:
		return *v.WeightGram
	case v.Size != nil:
		return v.Size.WeightGram
	case v.Product != nil:
		return v.Product.WeightGram
	}
	return 0
}

// ConcentrationLabel: "murni" → "Murni".
func ConcentrationLabel(c string) string {
	if c == "" {
		return ""
	}
	return strings.ToUpper(c[:1]) + c[1:]
}

// VariantCode membuat kode varian otomatis (admin tidak perlu mengetik SKU):
// parfum  → "<kode wangian>-<ml>-<murni|campur>", mis. "annasui-dolly-girl-30-murni"
// general → "<kode produk>[-<slug label>]".
func VariantCode(productCode, scentCode string, volumeML int, concentration, labelSlug string) string {
	if scentCode != "" {
		return fmt.Sprintf("%s-%d-%s", scentCode, volumeML, concentration)
	}
	if labelSlug != "" {
		return productCode + "-" + labelSlug
	}
	return productCode
}
