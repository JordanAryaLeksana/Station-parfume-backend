package services

import (
	"backend/src/config"
	"backend/src/modules/products/models"
	"backend/src/repository"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

var validate = validator.New()

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("already exists")
	ErrValidation = errors.New("validation error")
)

// productQuery memuat listing lengkap dengan urutan yang sama seperti di Shopee.
func productQuery() *gorm.DB {
	return config.DB.
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order") }).
		Preload("Variations", func(db *gorm.DB) *gorm.DB { return db.Order("tier_index") }).
		Preload("Variations.Options", func(db *gorm.DB) *gorm.DB { return db.Order("option_index") }).
		Preload("Variations.Options.Aroma").
		Preload("Variations.Options.Bottle").
		Preload("Models", func(db *gorm.DB) *gorm.DB { return db.Where("is_active = ?", true).Order("id") }).
		Preload("ShopCategories").
		Where("status = ?", repository.ProductStatusActive)
}

// GetAllProducts mengembalikan semua listing aktif. productType opsional
// (perfume / empty_bottle / other).
func GetAllProducts(productType string) ([]models.ProductResponseDTO, error) {
	q := productQuery()
	if productType != "" {
		q = q.Where("product_type = ?", productType)
	}
	var products []repository.Product
	if err := q.Order("sort_order, id").Find(&products).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve products: %w", err)
	}
	out := make([]models.ProductResponseDTO, 0, len(products))
	for _, p := range products {
		out = append(out, toProductResponse(p))
	}
	return out, nil
}

func GetProductByID(id uint) (*models.ProductResponseDTO, error) {
	var p repository.Product
	err := productQuery().First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve product: %w", err)
	}
	resp := toProductResponse(p)
	return &resp, nil
}

func toProductResponse(p repository.Product) models.ProductResponseDTO {
	resp := models.ProductResponseDTO{Product: p}
	used := map[uint]bool{}
	for i, m := range p.Models {
		if i == 0 || m.Price < resp.PriceMin {
			resp.PriceMin = m.Price
		}
		if m.Price > resp.PriceMax {
			resp.PriceMax = m.Price
		}
		resp.TotalStock += m.Stock
		if m.Option1ID != nil {
			used[*m.Option1ID] = true
		}
		if m.Option2ID != nil {
			used[*m.Option2ID] = true
		}
	}
	// Pilihan tanpa model aktif (sudah dihapus dari listing Shopee) tidak ditampilkan.
	resp.Variations = make([]repository.ProductVariation, len(p.Variations))
	for i, v := range p.Variations {
		opts := make([]repository.ProductVariationOption, 0, len(v.Options))
		for _, o := range v.Options {
			if used[o.ID] {
				opts = append(opts, o)
			}
		}
		v.Options = opts
		resp.Variations[i] = v
	}
	return resp
}
