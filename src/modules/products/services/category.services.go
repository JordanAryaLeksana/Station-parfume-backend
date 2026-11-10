package services

import (
	"backend/src/config"
	"backend/src/repository"
	"fmt"
)

// CategoryWithCount = kategori + jumlah produk yang tampil di website.
type CategoryWithCount struct {
	repository.Category
	ProductCount int `json:"product_count"`
}

// GetAllCategories mengembalikan kategori aktif urut sort_order (diisi dari seed/categories.csv).
// Jumlah produk dihitung dengan aturan yang sama dengan daftar produk publik
// (hanya produk yang punya varian bisa dibeli).
func GetAllCategories() ([]CategoryWithCount, error) {
	var cats []repository.Category
	if err := config.DB.Where("is_active").Order("sort_order, id").Find(&cats).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve categories: %w", err)
	}
	products, err := GetAllProducts(ProductFilter{})
	if err != nil {
		return nil, err
	}
	count := map[uint]int{}
	for _, p := range products {
		if p.Category != nil {
			count[p.Category.ID]++
		}
	}
	out := make([]CategoryWithCount, 0, len(cats))
	for _, c := range cats {
		out = append(out, CategoryWithCount{Category: c, ProductCount: count[c.ID]})
	}
	return out, nil
}
