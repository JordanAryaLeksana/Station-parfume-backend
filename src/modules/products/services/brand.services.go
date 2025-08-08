package services

import (
	"backend/src/modules/products/models"
	"backend/src/repository"
	"fmt"
		"backend/src/config"
)

func CreateBrand(input *models.BrandRequestDTO) (*models.BrandResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	brand := repository.Brand{
		Name:        input.Name,
		Description: input.Description,
		Logo:        input.Logo,
	}
	if err := config.DB.Create(&brand).Error; err != nil {
		return nil, fmt.Errorf("failed to create brand: %v", err)
	}
	return &models.BrandResponseDTO{
		ID:          brand.ID,
		Name:        brand.Name,
		Description: brand.Description,
		Logo:       brand.Logo,
	}, nil
}

func GetAllBrands() ([]models.BrandResponseDTO, error) {
	var brands []repository.Brand
	if err := config.DB.Find(&brands).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve brands: %v", err)
	}
	var brandResponses []models.BrandResponseDTO
	for _, brand := range brands {
		brandResponses = append(brandResponses, models.BrandResponseDTO{
			ID:          brand.ID,
			Name:        brand.Name,
			Description: brand.Description,
			Logo:        brand.Logo,
		})
	}
	return brandResponses, nil
}

