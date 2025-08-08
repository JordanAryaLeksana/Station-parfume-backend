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

	"gorm.io/gorm"
)

func CreateBrand(input *models.BrandRequestDTO) (*models.BrandResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	var existingBrand repository.Brand
	if err := config.DB.Find(&existingBrand, "name = ?", input.Name).Error; err != nil {
		return nil, fmt.Errorf("failed to check if brand exists: %v", err)
	} else if (!errors.Is(err, gorm.ErrRecordNotFound)) {
		return nil, fmt.Errorf("failed to check if brand exists: %v", err)
	}
	brand := repository.Brand{
		Name:        input.Name,
		Description: input.Description,
		Logo:        input.Logo,
	}
	if err := config.DB.Create(&brand).Error; err != nil {
		return nil, fmt.Errorf("failed to create brand: %v", err)
	}
	config.RedisClient.Del(context.Background(), "brands:all")
	return &models.BrandResponseDTO{
		ID:          brand.ID,
		Name:        brand.Name,
		Description: brand.Description,
		Logo:       brand.Logo,
	}, nil
}

func GetAllBrands() ([]models.BrandResponseDTO, error) {
	ctx := context.Background()
	cachedKey := "brands:all"
	cachedBrands, err := config.RedisClient.Get(ctx, cachedKey).Result()
	if err == nil && cachedBrands != "" {
		var brands []models.BrandResponseDTO
		if err := json.Unmarshal([]byte(cachedBrands), &brands); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached brands: %v", err)
		}
		return brands, nil
	}
	var brands []repository.Brand
	if err := config.DB.Preload("Parfume").Find(&brands).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve brands: %v", err)
	}
	var brandResponses []models.BrandResponseDTO
	for _, brand := range brands {
		var parfumes []models.Parfume
		for _, parfume := range brand.Parfume {
			parfumes = append(parfumes, models.Parfume{
				ID:   parfume.ID,
				Name: parfume.Name,
			})
		}

		brandResponses = append(brandResponses, models.BrandResponseDTO{
			ID:          brand.ID,
			Name:        brand.Name,
			Description: brand.Description,
			Logo:        brand.Logo,
			Parfume:     parfumes,
			CreatedAt:   brand.CreatedAt,
			UpdatedAt:   brand.UpdatedAt,
		})
	}

	brandResponsesJSON, err := json.Marshal(brandResponses)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal brands: %v", err)
	}
	if err := config.RedisClient.Set(ctx, cachedKey, brandResponsesJSON, 15*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("failed to cache brands: %v", err)
	}
	return brandResponses, nil
}

func GetBrandByID(id uint) (*models.BrandResponseDTO, error) {
	ctx := context.Background()
	cachedKey := fmt.Sprintf("brand:%d", id)
	cachedBrand, err := config.RedisClient.Get(ctx, cachedKey).Result()
	if err == nil && cachedBrand != "" {
		var brand models.BrandResponseDTO
		if err := json.Unmarshal([]byte(cachedBrand), &brand); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached brand: %v", err)
		}
		return &brand, nil
	}
	var brand repository.Brand
	if err := config.DB.Preload("Parfume").First(&brand, id).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve brand: %v", err)
	}
	var parfumes []models.Parfume
	for _, parfume := range brand.Parfume {
		parfumes = append(parfumes, models.Parfume{
			ID:          parfume.ID,
			Name:        parfume.Name,
		})
	}
	brandResponse := models.BrandResponseDTO{
		ID:          brand.ID,
		Name:        brand.Name,
		Description: brand.Description,
		Logo:        brand.Logo,
		Parfume:     parfumes,
		CreatedAt:   brand.CreatedAt,
		UpdatedAt:   brand.UpdatedAt,
	}
	
	brandResponseJSON, err := json.Marshal(brandResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal brand response: %v", err)
	}
	if err := config.RedisClient.Set(ctx, cachedKey, brandResponseJSON, 15*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("failed to cache brand response: %v", err)
	}
	return &brandResponse, nil
}