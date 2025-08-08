package services

import (
	"backend/src/config"
	"backend/src/modules/products/models"
	"backend/src/repository"
	"fmt"
)



func CreateBottle(input *models.BottleRequestDTO) (*models.BottleResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	if err := config.DB.Find(&repository.Bottle{}, "name = ?", input.Name).Error; err == nil {
		return nil, fmt.Errorf("bottle with name %s already exists", input.Name)
	}

	bottle := repository.Bottle{
		Name:        input.Name,
		Description: input.Description,
		Image:       input.Image,
		Size:        input.Size,
		Price:       input.Price,
		TypeBottleID: input.TypeBottleID,
	}

	if err := config.DB.Create(&bottle).Error; err != nil {
		return nil, fmt.Errorf("failed to create bottle: %v", err)
	}

	return &models.BottleResponseDTO{
		ID:          bottle.ID,
		Name:        bottle.Name,
		Description: bottle.Description,
		Image:       bottle.Image,
		Size:       bottle.Size,
		Price:      bottle.Price,
		TypeBottle: models.TypeBottle{
			ID:   bottle.TypeBottleID,
			Name: bottle.TypeBottle.Name,
		},
		CreatedAt:   bottle.CreatedAt,
		UpdatedAt:   bottle.UpdatedAt,
	}, nil
}

func GetAllBottles() ([]models.BottleResponseDTO, error) {
	var bottles []repository.Bottle
	if err := config.DB.Preload("TypeBottle").Find(&bottles).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve bottles: %v", err)
	}
	var bottleResponses []models.BottleResponseDTO
	for _, bottle := range bottles {
		bottleResponses = append(bottleResponses, models.BottleResponseDTO{
			ID:          bottle.ID,
			Name:        bottle.Name,
			Description: bottle.Description,
			Image:       bottle.Image,
			Size:        bottle.Size,
			Price:       bottle.Price,
			TypeBottle: models.TypeBottle{
				ID:   bottle.TypeBottleID,
				Name: bottle.TypeBottle.Name,
			},
		})
	}
	return bottleResponses, nil
}
func GetBottleByID(id uint) (*models.BottleResponseDTO, error) {
	var bottle repository.Bottle
	if err := config.DB.Preload("TypeBottle").First(&bottle, id).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve bottle with ID %d: %v", id, err)
	}
	return &models.BottleResponseDTO{
		ID:          bottle.ID,
		Name:        bottle.Name,
		Description: bottle.Description,
		Image:       bottle.Image,
		Size:        bottle.Size,
		Price:       bottle.Price,
		TypeBottle: models.TypeBottle{
			ID:   bottle.TypeBottleID,
			Name: bottle.TypeBottle.Name,
		},
		CreatedAt:   bottle.CreatedAt,
		UpdatedAt:   bottle.UpdatedAt,
	}, nil
}

func UpdateBottle(id uint, input *models.BottleRequestDTO) (*models.BottleResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	var bottle repository.Bottle
	if err := config.DB.First(&bottle, id).Error; err != nil {
		return nil, fmt.Errorf("failed to find bottle with ID %d: %v", id, err)
	}
	bottle.Name = input.Name
	bottle.Description = input.Description
	bottle.Image = input.Image
	bottle.Size = input.Size
	bottle.Price = input.Price
	bottle.TypeBottleID = input.TypeBottleID
	if err := config.DB.Save(&bottle).Error; err != nil {
		return nil, fmt.Errorf("failed to update bottle: %v", err)
	}
	return &models.BottleResponseDTO{
		ID:          bottle.ID,
		Name:        bottle.Name,
		Description: bottle.Description,
		Image:       bottle.Image,
		Size:        bottle.Size,
		Price:       bottle.Price,
		TypeBottle: models.TypeBottle{
			ID:   bottle.TypeBottleID,
			Name: bottle.TypeBottle.Name,
		},
		CreatedAt:   bottle.CreatedAt,
		UpdatedAt:   bottle.UpdatedAt,
	}, nil
}

func DeleteBottle(id uint) error {
	var bottle repository.Bottle
	if err := config.DB.First(&bottle, id).Error; err != nil {
		return fmt.Errorf("failed to find bottle with ID %d: %v", id, err)
	}
	if err := config.DB.Delete(&bottle).Error; err != nil {
		return fmt.Errorf("failed to delete bottle: %v", err)
	}
	return nil
}