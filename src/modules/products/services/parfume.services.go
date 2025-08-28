package services

import (
	"backend/src/config"
	"backend/src/modules/products/models"
	"backend/src/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

var validate = validator.New()

func CreateParfume(items models.ParfumeRequestDTO) (*models.ParfumeResponseDTO, error) {
	fmt.Println("Memulai service CreateParfume")
	if err := validate.Struct(items); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	var existingParfume repository.Parfume
	if err := config.DB.Find(&existingParfume, "name = ?", items.Name).Error; err == nil {
		return nil, fmt.Errorf("parfume with name %s already exists", items.Name)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check if parfume exists: %v", err)
	}
	var brand repository.Brand
	if err := config.DB.First(&brand, items.BrandID).Error; err != nil {
		return nil, fmt.Errorf("brand not found with ID %d: %v", items.BrandID, err)
	}
	var Type repository.Type
	if err := config.DB.First(&Type, items.TypeID).Error; err != nil {
		return nil, fmt.Errorf("type not found with ID %d: %v", items.TypeID, err)
	}
	var Category repository.Categories
	if err := config.DB.First(&Category, items.CategoryID).Error; err != nil {
		return nil, fmt.Errorf("category not found with ID %d: %v", items.CategoryID, err)
	}
	parfume := repository.Parfume{
		Name:        items.Name,
		Description: items.Description,
		PriceML:       items.Price,
		Image:       items.Image,
		Type:        Type,
		Category:    Category,
		Favorite:    items.Favorite,
		IsNew:       items.IsNew,
	}
	if err := config.DB.Create(&parfume).Error; err != nil {
		return nil, fmt.Errorf("failed to create parfume: %v", err)
	}
	config.RedisClient.Del(context.Background(), "parfumes:all") // Clear cache for all parfumes
	response := models.ParfumeResponseDTO{
		ID:          parfume.ID,
		Name:        parfume.Name,
		Description: parfume.Description,
		Price:       parfume.PriceML,
		Image:       parfume.Image,
		Type:        models.TypeDTO(parfume.Type),
		Category:    models.CategoriesDTO(parfume.Category),
		Favorite:    parfume.Favorite,
		IsNew: 	 parfume.IsNew,
		Brand: models.BrandDTO{
			ID:          brand.ID,
			Name:        brand.Name,
			Logo:        brand.Logo,
			Description: brand.Description,
		},
	}
	return &response, nil
}
func AddParfumeToBrand(brandID uint, dto models.ParfumeRequestDTO) (*models.ParfumeResponseDTO, error) {
    if err := validate.Struct(dto); err != nil {
        return nil, fmt.Errorf("validation failed: %v", err)
    }
    var brand repository.Brand
    if err := config.DB.First(&brand, brandID).Error; err != nil {
        return nil, fmt.Errorf("brand with ID %d not found: %v", brandID, err)
    }
    var (
        category repository.Categories
        ptype    repository.Type
    )
    if err := config.DB.First(&category, dto.CategoryID).Error; err != nil {
        return nil, fmt.Errorf("category with ID %d not found: %v", dto.CategoryID, err)
    }
    if err := config.DB.First(&ptype, dto.TypeID).Error; err != nil {
        return nil, fmt.Errorf("type with ID %d not found: %v", dto.TypeID, err)
    }

    parfume := repository.Parfume{
        Name:        dto.Name,
        Description: dto.Description,
        PriceML:       dto.Price,
        Image:       dto.Image,
        Favorite:    dto.Favorite,
        BrandID:     brandID,
        TypeID:      dto.TypeID,
        CategoryID:  dto.CategoryID,
        IsNew:       dto.IsNew,
    }

    if err := config.DB.Create(&parfume).Error; err != nil {
        return nil, fmt.Errorf("failed to create parfume: %v", err)
    }

    // 5. Response DTO
    response := models.ParfumeResponseDTO{
        ID:          parfume.ID,
        Name:        parfume.Name,
        Description: parfume.Description,
        Price:       parfume.PriceML,
        Image:       parfume.Image,
        Favorite:    parfume.Favorite,
        Type:        models.TypeDTO(ptype),
        Category:    models.CategoriesDTO(category),
		IsNew:  dto.IsNew,
        Brand: models.BrandDTO{
            ID:          brand.ID,
            Name:        brand.Name,
            Logo:        brand.Logo,
            Description: brand.Description,
        },
    }

    return &response, nil
}

func GetAllParfumes() ([]models.ParfumeResponseDTO, error) {
	ctx := context.Background()
	cachedKey := "parfumes:all"
	cachedParfumes, err := config.RedisClient.Get(ctx, cachedKey).Result()
	if err == nil && cachedParfumes != "" {
		var parfumes []models.ParfumeResponseDTO
		if err := json.Unmarshal([]byte(cachedParfumes), &parfumes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached parfumes: %v", err)
		}
		return parfumes, nil
	}
	var parfumes []repository.Parfume
	if err := config.DB.Preload("Type").Preload("Category").Preload("Brand").Find(&parfumes).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve parfumes: %v", err)
	}
	var response []models.ParfumeResponseDTO
	for _, parfume := range parfumes {
		response = append(response, models.ParfumeResponseDTO{
			ID:          parfume.ID,
			Name:        parfume.Name,
			Description: parfume.Description,
			Price:       parfume.PriceML,
			Image:       parfume.Image,
			Type:        models.TypeDTO(parfume.Type),
			Category:    models.CategoriesDTO(parfume.Category),
			Favorite:    parfume.Favorite,
			IsNew:       parfume.IsNew,
			Brand: models.BrandDTO{
				ID:          parfume.Brand.ID,
				Name:        parfume.Brand.Name,
				Logo:        parfume.Brand.Logo,
				Description: parfume.Brand.Description,
			},
		})
	}
	brandResponseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parfumes: %v", err)
	}
	if err := config.RedisClient.Set(ctx, cachedKey, brandResponseJSON, 15*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("failed to cache parfumes: %v", err)
	}
	return response, nil
}

func GetParfumeById(id string) (*models.ParfumeResponseDTO, error) {
	ctx := context.Background()
	cachedKey := fmt.Sprintf("parfume:%s", id)
	cachedParfume, err := config.RedisClient.Get(ctx, cachedKey).Result()
	if err == nil && cachedParfume != "" {
		var parfume models.ParfumeResponseDTO
		if err := json.Unmarshal([]byte(cachedParfume), &parfume); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached parfume: %v", err)
		}
		return &parfume, nil
	}
	parfume := repository.Parfume{}
	if err := config.DB.Preload("Type").Preload("Category").Preload("Brand").First(&parfume, id).Error; err != nil {
		return nil, fmt.Errorf("parfume not found with id: %s, error: %v", id, err)
	}

	response := models.ParfumeResponseDTO{
		ID:          parfume.ID,
		Name:        parfume.Name,
		Description: parfume.Description,
		Price:       parfume.PriceML,
		Image:       parfume.Image,
		Type:        models.TypeDTO(parfume.Type),
		Category:    models.CategoriesDTO(parfume.Category),
		Favorite:    parfume.Favorite,
		IsNew: 	 parfume.IsNew,
		Brand: models.BrandDTO{
			ID:          parfume.Brand.ID,
			Name:        parfume.Brand.Name,
			Logo:        parfume.Brand.Logo,
			Description: parfume.Brand.Description,
		},
	}
	parfumeJSON, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parfume response: %v", err)
	}
	if err := config.RedisClient.Set(ctx, cachedKey, parfumeJSON, 15*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("failed to cache parfume response: %v", err)
	}
	
	return &response, nil
}

func UpdateParfume(id string, model models.ParfumeRequestDTO) (*models.ParfumeResponseDTO, error) {
	var parfume repository.Parfume
	if err := config.DB.Preload("Type").Preload("Category").Preload("Brand").First(&parfume, id).Error; err != nil {
		return nil, fmt.Errorf("parfume not found with id: %s", id)
	}
	if err := validate.Struct(model); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	var brand repository.Brand
	if err := config.DB.First(&brand, model.BrandID).Error; err != nil {
		return nil, fmt.Errorf("brand not found with ID %d: %v", model.BrandID, err)
	}
	var Type repository.Type
	if err := config.DB.First(&Type, model.TypeID).Error; err != nil {
		return nil, fmt.Errorf("type not found with ID %d: %v", model.TypeID, err)
	}
	var Category repository.Categories
	if err := config.DB.First(&Category, model.CategoryID).Error; err != nil {
		return nil, fmt.Errorf("category not found with ID %d: %v", model.CategoryID, err)
	}
	parfume.Name = model.Name
	parfume.Description = model.Description
	parfume.PriceML = model.Price
	parfume.Image = model.Image
	parfume.Type = Type
	parfume.Category = Category
	parfume.IsNew = model.IsNew
	parfume.Favorite = model.Favorite
	if err := config.DB.Save(&parfume).Error; err != nil {
		return nil, fmt.Errorf("failed to update parfume: %v", err)
	}
	response := models.ParfumeResponseDTO{
		ID:          parfume.ID,
		Name:        parfume.Name,
		Description: parfume.Description,
		Price:       parfume.PriceML,
		Image:       parfume.Image,
		Type:        models.TypeDTO(parfume.Type),
		Category:    models.CategoriesDTO(parfume.Category),
		Favorite:    parfume.Favorite,
		IsNew:       parfume.IsNew,
		Brand: models.BrandDTO{
			ID:          brand.ID,
			Name:        brand.Name,
			Logo:        brand.Logo,
			Description: brand.Description,
		},
	}
	config.RedisClient.Del(context.Background(), "parfumes:all") // Clear cache for all parfumes
	config.RedisClient.Del(context.Background(), fmt.Sprintf("parfume:%s", id)) // Clear cache for specific parfume
	return &response, nil
}

func SortParfumeByType(parfumeType string) ([]models.ParfumeResponseDTO, error) {
	var parfumes []repository.Parfume
	if err := config.DB.Joins("JOIN types ON types.id = parfumes.type_id").Where("types.name = ?", parfumeType).Preload("Type").Preload("Category").Preload("Brand").Find(&parfumes).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve parfumes by type %s: %v", parfumeType, err)
	}
	if len(parfumes) == 0 {
		return nil, fmt.Errorf("no parfumes found for type %s", parfumeType)
	}
	var response []models.ParfumeResponseDTO
	for _, parfume := range parfumes {
		response = append(response, models.ParfumeResponseDTO{
			ID:          parfume.ID,
			Name:        parfume.Name,
			Description: parfume.Description,
			Price:       parfume.PriceML,
			Image:       parfume.Image,
			Type:        models.TypeDTO(parfume.Type),
			Category:    models.CategoriesDTO(parfume.Category),
			Favorite:    parfume.Favorite,
			IsNew:  parfume.IsNew,
			Brand: models.BrandDTO{
				ID:          parfume.Brand.ID,
				Name:        parfume.Brand.Name,
				Logo:        parfume.Brand.Logo,
				Description: parfume.Brand.Description,
			},
		})
	}
	return response, nil

}

func SortParfumesByBrand(brandName string) ([]models.ParfumeResponseDTO, error) {
	var parfumes []repository.Parfume
	if err := config.DB.Joins("JOIN brands ON brands.id = parfumes.brand_id").Where("LOWER(brands.name) = LOWER(?)", brandName).Preload("Type").Preload("Category").Order("parfumes.name ASC").Preload("Brand").Find(&parfumes).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve parfumes by brand %s: %v", brandName, err)
	}
	if len(parfumes) == 0 {
		return nil, fmt.Errorf("no parfumes found for brand %s", brandName)
	}
	var response []models.ParfumeResponseDTO
	for _, parfume := range parfumes {
		response = append(response, models.ParfumeResponseDTO{
			ID:          parfume.ID,
			Name:        parfume.Name,
			Description: parfume.Description,
			Price:       parfume.PriceML,
			Image: 	 parfume.Image,	
			Type:        models.TypeDTO(parfume.Type),
			Category:    models.CategoriesDTO(parfume.Category),
			Favorite:    parfume.Favorite,
			IsNew:       parfume.IsNew,
			Brand: models.BrandDTO{
				ID:          parfume.Brand.ID,
				Name:        parfume.Brand.Name,
				Logo:        parfume.Brand.Logo,
				Description: parfume.Brand.Description,
			},
		})
	}
	return response, nil
}

func SortParfumesByCategory(category string) ([]models.ParfumeResponseDTO, error) {
	var parfumes []repository.Parfume
	if err := config.DB.Joins("JOIN categories ON categories.id = parfumes.category_id").Where("categories.name = ?", category).Preload("Type").Preload("Category").Preload("Brand").Find(&parfumes).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve parfumes by category %s: %v", category, err)
	}

	if len(parfumes) == 0 {
		return nil, fmt.Errorf("no parfumes found for category %s", category)
	} else {
		var response []models.ParfumeResponseDTO
		for _, parfume := range parfumes {
			response = append(response, models.ParfumeResponseDTO{
				ID:          parfume.ID,
				Name:        parfume.Name,
				Description: parfume.Description,
				Price:       parfume.PriceML,
				Image:       parfume.Image,
				Type:        models.TypeDTO(parfume.Type),
				Category:    models.CategoriesDTO(parfume.Category),
				Favorite:    parfume.Favorite,
				IsNew:       parfume.IsNew,
				Brand: models.BrandDTO{
					ID:          parfume.Brand.ID,
					Name:        parfume.Brand.Name,
					Logo:        parfume.Brand.Logo,
					Description: parfume.Brand.Description,
				},
			})
		}
		return response, nil
	}
}

