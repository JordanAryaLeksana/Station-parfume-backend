package services

import (
	"backend/src/config"
	"backend/src/modules/products/models"
	"backend/src/repository"
	"backend/src/utils/slug"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------
// Aroma (master data, diisi dari seed/aromas.csv)
// ---------------------------------------------------------------------

func GetAllAromas() ([]repository.Aroma, error) {
	var aromas []repository.Aroma
	if err := config.DB.Preload("Brand").Where("is_active = ?", true).Order("id").Find(&aromas).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve aromas: %w", err)
	}
	return aromas, nil
}

func GetAromaByID(id uint) (*repository.Aroma, error) {
	var a repository.Aroma
	if err := config.DB.Preload("Brand").First(&a, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func UpdateAroma(id uint, input *models.AromaUpdateDTO) (*repository.Aroma, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	var a repository.Aroma
	if err := config.DB.First(&a, id).Error; err != nil {
		return nil, notFound(err)
	}
	updates := map[string]any{}
	if input.BrandID != nil {
		if *input.BrandID == 0 {
			updates["brand_id"] = nil
		} else {
			var b repository.Brand
			if err := config.DB.First(&b, *input.BrandID).Error; err != nil {
				return nil, fmt.Errorf("brand %d: %w", *input.BrandID, notFound(err))
			}
			updates["brand_id"] = b.ID
		}
	}
	setIfNotNil(updates, "gender", input.Gender)
	setIfNotNil(updates, "grade", input.Grade)
	setIfNotNil(updates, "description", input.Description)
	setIfNotNil(updates, "top_notes", input.TopNotes)
	setIfNotNil(updates, "middle_notes", input.MiddleNotes)
	setIfNotNil(updates, "base_notes", input.BaseNotes)
	setIfNotNil(updates, "image_url", input.ImageURL)
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}
	if len(updates) > 0 {
		if err := config.DB.Model(&a).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update aroma: %w", err)
		}
	}
	return GetAromaByID(id)
}

// ---------------------------------------------------------------------
// Brand (brand inspirasi aroma)
// ---------------------------------------------------------------------

func GetAllBrands() ([]repository.Brand, error) {
	var brands []repository.Brand
	if err := config.DB.Order("name").Find(&brands).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve brands: %w", err)
	}
	return brands, nil
}

func CreateBrand(input *models.BrandRequestDTO) (*repository.Brand, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	s := slug.Make(input.Name)
	if err := ensureSlugFree(&repository.Brand{}, s, 0); err != nil {
		return nil, err
	}
	b := repository.Brand{Name: input.Name, Slug: s, Description: input.Description, LogoURL: input.LogoURL}
	if err := config.DB.Create(&b).Error; err != nil {
		return nil, fmt.Errorf("failed to create brand: %w", err)
	}
	return &b, nil
}

// ---------------------------------------------------------------------
// Bottle (master data botol)
// ---------------------------------------------------------------------

func GetAllBottles() ([]repository.Bottle, error) {
	var bottles []repository.Bottle
	if err := config.DB.Where("is_active = ?", true).Order("volume_ml, id").Find(&bottles).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve bottles: %w", err)
	}
	return bottles, nil
}

func GetBottleByID(id uint) (*repository.Bottle, error) {
	var b repository.Bottle
	if err := config.DB.First(&b, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &b, nil
}

func CreateBottle(input *models.BottleRequestDTO) (*repository.Bottle, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	s := slug.Make(input.Name)
	if err := ensureSlugFree(&repository.Bottle{}, s, 0); err != nil {
		return nil, err
	}
	b := repository.Bottle{
		Name: input.Name, Slug: s, BottleType: input.BottleType, VolumeML: input.VolumeML,
		Description: input.Description, ImageURL: input.ImageURL, IsActive: input.IsActive == nil || *input.IsActive,
	}
	if err := config.DB.Create(&b).Error; err != nil {
		return nil, fmt.Errorf("failed to create bottle: %w", err)
	}
	return &b, nil
}

func UpdateBottle(id uint, input *models.BottleRequestDTO) (*repository.Bottle, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	b, err := GetBottleByID(id)
	if err != nil {
		return nil, err
	}
	s := slug.Make(input.Name)
	if err := ensureSlugFree(&repository.Bottle{}, s, b.ID); err != nil {
		return nil, err
	}
	updates := map[string]any{
		"name": input.Name, "slug": s, "bottle_type": input.BottleType, "volume_ml": input.VolumeML,
		"description": input.Description, "image_url": input.ImageURL,
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}
	if err := config.DB.Model(b).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update bottle: %w", err)
	}
	return GetBottleByID(id)
}

// ---------------------------------------------------------------------

// ensureSlugFree memastikan slug belum dipakai baris aktif lain (selain exceptID).
func ensureSlugFree(model any, s string, exceptID uint) error {
	if s == "" {
		return fmt.Errorf("%w: name harus mengandung huruf atau angka", ErrValidation)
	}
	var count int64
	q := config.DB.Model(model).Where("slug = ?", s)
	if exceptID != 0 {
		q = q.Where("id <> ?", exceptID)
	}
	if err := q.Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check slug: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("slug %q: %w", s, ErrConflict)
	}
	return nil
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func setIfNotNil(m map[string]any, col string, v *string) {
	if v != nil {
		m[col] = *v
	}
}
