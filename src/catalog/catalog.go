// Package catalog berisi aturan bersama katalog yang dipakai modul produk,
// keranjang, dan order: varian mana yang bisa dibeli, harganya, dan stoknya.
package catalog

import (
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
	Total       int64  // harga jual 1 unit
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

// StockOf: stok tersedia varian. nil = stok tidak dihitung (products.track_stock
// = false, mis. parfum diracik sendiri) → selalu tersedia. Product harus di-Preload.
func StockOf(v *repository.Variant) *int {
	if v.Product != nil && !v.Product.TrackStock {
		return nil
	}
	n := v.Available()
	return &n
}

// HasStock: varian bisa dibeli sebanyak qty (selalu true kalau stok tidak dihitung).
func HasStock(v *repository.Variant, qty int) bool {
	s := StockOf(v)
	return s == nil || *s >= qty
}

// UnitPrice: harga 1 unit varian; 0 kalau harganya belum bisa dihitung
// (varian seperti itu tidak bisa dibeli, lihat Purchasable).
func UnitPrice(v *repository.Variant) int64 {
	price, ok := ListPrice(v)
	if !ok {
		return 0
	}
	return price.Total
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
