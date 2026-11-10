// Package catalog berisi aturan bersama katalog yang dipakai modul produk,
// keranjang, dan order: varian mana yang bisa dibeli, harga untuk pembeli
// (termasuk reseller), dan stok tersedia di lokasi default.
package catalog

import (
	"errors"
	"fmt"

	"backend/src/repository"

	"gorm.io/gorm"
)

// PreloadVariant memuat semua yang dibutuhkan untuk menampilkan / membeli varian.
func PreloadVariant(db *gorm.DB, prefix string) *gorm.DB {
	if prefix != "" {
		prefix += "."
	}
	return db.Preload(prefix+"Product").
		Preload(prefix+"Product.Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order") }).
		Preload(prefix + "Product.Sizes").
		Preload(prefix + "Scent").
		Preload(prefix + "Size")
}

// Price = rincian harga normal 1 unit varian.
type Price struct {
	Total       int64  // harga normal (sebelum reseller)
	PerML       *int64 // tarif per ml yang dipakai (nil = harga khusus / produk general)
	BottlePrice *int64 // harga botol yang dipakai
}

// ListPrice menghitung harga normal varian:
//   - variants.price diisi → itu harganya (harga khusus parfum / harga produk general);
//   - parfum → tarif per ml wangian × volume + harga botol ukuran.
//
// ok=false kalau harga belum bisa dihitung (tarif kosong, Scent/Size belum di-Preload).
func ListPrice(v *repository.Variant) (Price, bool) {
	if v == nil {
		return Price{}, false
	}
	if v.Price != nil {
		return Price{Total: *v.Price}, true
	}
	if v.Scent == nil || v.Size == nil || v.Concentration == nil {
		return Price{}, false
	}
	rate := v.Scent.Rate(*v.Concentration)
	if rate == nil {
		return Price{}, false
	}
	bottle := v.Size.BottlePrice
	return Price{Total: *rate*int64(v.Size.VolumeML) + bottle, PerML: rate, BottlePrice: &bottle}, true
}

// Purchasable: varian aktif, harganya bisa dihitung, produk dijual, wangian & ukuran aktif.
func Purchasable(v *repository.Variant) bool {
	if v == nil || !v.IsActive || v.Product == nil || v.Product.Status != repository.ProductStatusActive {
		return false
	}
	if _, ok := ListPrice(v); !ok {
		return false
	}
	if v.Scent != nil && !v.Scent.IsActive {
		return false
	}
	if v.Size != nil && !SizeAllowed(v.Product, v.Size) {
		return false
	}
	return true
}

// SizeAllowed: ukuran aktif dan termasuk ukuran yang dijual produk ini — daftar
// khusus (Product.Sizes, harus di-Preload) kalau ada, kalau tidak ukuran default.
func SizeAllowed(p *repository.Product, size *repository.Size) bool {
	if !size.IsActive {
		return false
	}
	if len(p.Sizes) == 0 {
		return size.IsDefault
	}
	for _, s := range p.Sizes {
		if s.ID == size.ID {
			return true
		}
	}
	return false
}

// DefaultLocationID: lokasi stok yang dipakai checkout (sekarang 1 toko).
func DefaultLocationID(db *gorm.DB) (uint, error) {
	var loc repository.Location
	err := db.Where("is_default AND is_active").First(&loc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("lokasi stok default belum ada (jalankan seeder)")
	}
	return loc.ID, err
}

// Available mengembalikan stok tersedia (on_hand − reserved) per varian di lokasi.
// Varian tanpa baris inventory = 0.
func Available(db *gorm.DB, locationID uint, variantIDs []uint) (map[uint]int, error) {
	out := make(map[uint]int, len(variantIDs))
	if len(variantIDs) == 0 {
		return out, nil
	}
	var levels []repository.InventoryLevel
	if err := db.Where("location_id = ? AND variant_id IN ?", locationID, variantIDs).Find(&levels).Error; err != nil {
		return nil, err
	}
	for _, l := range levels {
		out[l.VariantID] = l.Available()
	}
	return out, nil
}

// StockOf: stok tersedia varian. nil = stok tidak dihitung (products.track_stock
// = false, mis. parfum diracik sendiri) → selalu tersedia. Product harus di-Preload.
func StockOf(v *repository.Variant, avail map[uint]int) *int {
	if v.Product != nil && !v.Product.TrackStock {
		return nil
	}
	n := max(avail[v.ID], 0)
	return &n
}

// HasStock: varian bisa dibeli sebanyak qty (selalu true kalau stok tidak dihitung).
func HasStock(v *repository.Variant, avail map[uint]int, qty int) bool {
	s := StockOf(v, avail)
	return s == nil || *s >= qty
}

// Pricing = konteks harga untuk 1 pembeli (reseller atau bukan).
type Pricing struct {
	Level     *repository.ResellerLevel
	overrides map[uint]int64 // variant_id → harga khusus level ini
}

// PricingForUser memuat level reseller user (kalau ada & aktif) beserta harga
// khusus untuk varian yang diminta.
func PricingForUser(db *gorm.DB, userID uint, variantIDs []uint) (*Pricing, error) {
	p := &Pricing{overrides: map[uint]int64{}}
	if userID == 0 {
		return p, nil
	}
	var u repository.User
	if err := db.Preload("ResellerLevel").First(&u, userID).Error; err != nil {
		return nil, err
	}
	if u.ResellerLevel == nil || !u.ResellerLevel.IsActive {
		return p, nil
	}
	p.Level = u.ResellerLevel
	if len(variantIDs) > 0 {
		var rows []repository.VariantResellerPrice
		if err := db.Where("reseller_level_id = ? AND variant_id IN ?", p.Level.ID, variantIDs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			p.overrides[r.VariantID] = r.Price
		}
	}
	return p, nil
}

// UnitPrice: harga yang dibayar pembeli untuk 1 unit varian.
// Urutan: harga khusus reseller → harga normal − diskon % level → harga normal.
// Varian yang harganya belum bisa dihitung mengembalikan 0 (tidak bisa dibeli).
func (p *Pricing) UnitPrice(v *repository.Variant) int64 {
	price, ok := ListPrice(v)
	if !ok {
		return 0
	}
	if p == nil || p.Level == nil {
		return price.Total
	}
	if o, ok := p.overrides[v.ID]; ok {
		return o
	}
	return ApplyDiscount(price.Total, int(p.Level.DiscountPercent))
}

// SizesForProduct: ukuran aktif yang dijual untuk 1 produk parfum — daftar
// khusus di product_sizes kalau ada, kalau tidak semua ukuran default.
func SizesForProduct(db *gorm.DB, productID uint) ([]repository.Size, error) {
	var sizes []repository.Size
	err := db.Joins("JOIN product_sizes ps ON ps.size_id = sizes.id AND ps.product_id = ?", productID).
		Where("sizes.is_active").Order("sizes.volume_ml").Find(&sizes).Error
	if err != nil || len(sizes) > 0 {
		return sizes, err
	}
	err = db.Where("is_active AND is_default").Order("volume_ml").Find(&sizes).Error
	return sizes, err
}

// ApplyDiscount: harga − percent% dibulatkan ke bawah ke rupiah, minimal Rp1.
func ApplyDiscount(price int64, percent int) int64 {
	if percent <= 0 {
		return price
	}
	out := price * int64(100-percent) / 100
	if out < 1 {
		return 1
	}
	return out
}
