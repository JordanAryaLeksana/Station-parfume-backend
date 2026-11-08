package services

import (
	"backend/src/config"
	"backend/src/repository"
	"fmt"
)

// CategoryWithCount = kategori toko + jumlah produk aktif, seperti di halaman
// toko Shopee ("Parfum & Wewangian (75)").
type CategoryWithCount struct {
	repository.Category
	ProductCount int64 `json:"product_count"`
}

// GetAllCategories mengembalikan kategori aktif dengan urutan yang sama seperti di Shopee.
// Isinya diatur lewat seed/categories.csv, bukan lewat API.
func GetAllCategories() ([]CategoryWithCount, error) {
	var out []CategoryWithCount
	err := config.DB.Model(&repository.Category{}).
		Select("categories.*, COUNT(p.id) AS product_count").
		Joins("LEFT JOIN products p ON p.category_id = categories.id AND p.status = ? AND p.deleted_at IS NULL", repository.ProductStatusActive).
		Where("categories.is_active").
		Group("categories.id").
		Order("categories.sort_order, categories.id").
		Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve categories: %w", err)
	}
	return out, nil
}
