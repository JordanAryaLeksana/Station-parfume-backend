package services

import (
	"backend/src/config"
	"backend/src/modules/parfume/models"
	"backend/src/repository"
	"fmt"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func CreateParfume(items models.ParfumeRequestDTO) (*models.ParfumeResponseDTO, error) {
	fmt.Println("Memulai service CreateParfume")
	if err := validate.Struct(items); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
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
		Price:       items.Price,
		Image:       items.Image,
		Type:        Type,
		Category:    Category,
		Favorite:    items.Favorite,
	}
	if err := config.DB.Create(&parfume).Error; err != nil {
		return nil, fmt.Errorf("failed to create parfume: %v", err)
	}
	response := models.ParfumeResponseDTO{
		ID:          parfume.ID,
		Name:        parfume.Name,
		Description: parfume.Description,
		Price:       parfume.Price,
		Image:       parfume.Image,
		Type:        models.TypeDTO(parfume.Type),
		Category:    models.CategoriesDTO(parfume.Category),
		Favorite:    parfume.Favorite,
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
			Price:       parfume.Price,
			Image:       parfume.Image,
			Type:        models.TypeDTO(parfume.Type),
			Category:    models.CategoriesDTO(parfume.Category),
			Favorite:    parfume.Favorite,
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

func GetParfumeById(id string) (*models.ParfumeResponseDTO, error) {
	parfume := repository.Parfume{}
	if err := config.DB.Preload("Type").Preload("Category").Preload("Brand").First(&parfume, id).Error; err != nil {
		return nil, fmt.Errorf("parfume not found with id: %s, error: %v", id, err)
	}
	response := models.ParfumeResponseDTO{
		ID:          parfume.ID,
		Name:        parfume.Name,
		Description: parfume.Description,
		Price:       parfume.Price,
		Image:       parfume.Image,
		Type:        models.TypeDTO(parfume.Type),
		Category:    models.CategoriesDTO(parfume.Category),
		Favorite:    parfume.Favorite,
		Brand: models.BrandDTO{
			ID:          parfume.Brand.ID,
			Name:        parfume.Brand.Name,
			Logo:        parfume.Brand.Logo,
			Description: parfume.Brand.Description,
		},
	}
	return &response, nil
}

func UpdateParfume(id string, model models.ParfumeRequestDTO) (*models.ParfumeResponseDTO, error) {
	var parfume repository.Parfume 
	if err := config.DB.First(&parfume, id).Error; err != nil {
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
	parfume.Price = model.Price
	parfume.Image = model.Image
	parfume.Type = Type
	parfume.Category = Category
	parfume.Favorite = model.Favorite
	if err := config.DB.Save(&parfume).Error; err != nil {
		return nil, fmt.Errorf("failed to update parfume: %v", err)
	}
	response := models.ParfumeResponseDTO{
		ID:          parfume.ID,
		Name:        parfume.Name,
		Description: parfume.Description,
		Price:       parfume.Price,
		Image:       parfume.Image,
		Type:        models.TypeDTO(parfume.Type),
		Category:    models.CategoriesDTO(parfume.Category),
		Favorite:    parfume.Favorite,
		Brand: models.BrandDTO{
			ID:          brand.ID,
			Name:        brand.Name,
			Logo:        brand.Logo,
			Description: brand.Description,
		},
	}
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
			Price:       parfume.Price,
			Image:       parfume.Image,
			Type:        models.TypeDTO(parfume.Type),
			Category:    models.CategoriesDTO(parfume.Category),
			Favorite:    parfume.Favorite,
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
				Price:       parfume.Price,
				Image:       parfume.Image,
				Type:        models.TypeDTO(parfume.Type),
				Category:    models.CategoriesDTO(parfume.Category),
				Favorite:    parfume.Favorite,
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

