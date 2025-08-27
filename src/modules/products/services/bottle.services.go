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



func CreateBottle(input *models.BottleRequestDTO) (*models.BottleResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	var existingBottle repository.Bottle
	if err := config.DB.Find(&existingBottle, "name = ?", input.Name).Error; err == nil {
		return nil, fmt.Errorf("bottle with name %s already exists", input.Name)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check if bottle exists: %v", err)
	}

	bottle := repository.Bottle{
		Name:        input.Name,
		Description: input.Description,
		Image:       input.Image,
		Size:        input.Size,
		Price:       input.Price,
		IsNew:       input.IsNew,
		IsFavorite:  input.IsFavorite,
		TypeBottleID: input.TypeBottleID,
	}

	if err := config.DB.Create(&bottle).Error; err != nil {
		return nil, fmt.Errorf("failed to create bottle: %v", err)
	}

	config.RedisClient.Del(context.Background(), "bottles:all") 
	if err := config.DB.Preload("TypeBottle").First(&bottle, bottle.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve created bottle: %v", err)
	}

	
	return &models.BottleResponseDTO{
		ID:          bottle.ID,
		Name:        bottle.Name,
		Description: bottle.Description,
		Image:       bottle.Image,
		Size:       bottle.Size,
		Price:      bottle.Price,
		IsNew:      bottle.IsNew,
		IsFavorite: bottle.IsFavorite,
		TypeBottle: models.TypeBottle{
			ID:   bottle.TypeBottleID,
			Name: bottle.TypeBottle.Name,
		},
		CreatedAt:   bottle.CreatedAt,
		UpdatedAt:   bottle.UpdatedAt,
	}, nil
}

func GetAllBottles() ([]models.BottleResponseDTO, error) {
	ctx := context.Background()
	cachedKey := "bottles:all"
	cachedBottles, err := config.RedisClient.Get(ctx, cachedKey).Result()
	if (err == nil && cachedBottles != "") {
		var bottles []models.BottleResponseDTO
		if err := json.Unmarshal([]byte(cachedBottles), &bottles); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached bottles: %v", err)
		}
		return bottles, nil
	}

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
			IsNew:      bottle.IsNew,
			IsFavorite: bottle.IsFavorite,
			TypeBottle: models.TypeBottle{
				ID:   bottle.TypeBottleID,
				Name: bottle.TypeBottle.Name,
			},
		})
	}

	bottleResponsesJSON, err := json.Marshal(bottleResponses)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bottle responses: %v", err)
	}
	config.RedisClient.Set(ctx, cachedKey, bottleResponsesJSON, 15*time.Minute).Err()
	return bottleResponses, nil
}
func GetBottleByID(id uint) (*models.BottleResponseDTO, error) {
	ctx := context.Background()
	cachedKey := fmt.Sprintf("bottle:%d", id)

	cachedBottle, err := config.RedisClient.Get(ctx, cachedKey).Result()
	if err == nil && cachedBottle != "" {
		var bottle models.BottleResponseDTO
		if err := json.Unmarshal([]byte(cachedBottle), &bottle); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached bottle: %v", err)
		}
		return &bottle, nil
	}

	var bottle repository.Bottle
	if err := config.DB.Preload("TypeBottle").First(&bottle, id).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve bottle with ID %d: %v", id, err)
	}
	var bottleResponse = models.BottleResponseDTO{
		ID:          bottle.ID,
		Name:        bottle.Name,
		Description: bottle.Description,
		Image:       bottle.Image,
		Size:        bottle.Size,
		IsNew:      bottle.IsNew,
		IsFavorite: bottle.IsFavorite,
		Price:       bottle.Price,
		TypeBottle: models.TypeBottle{
			ID:   bottle.TypeBottleID,
			Name: bottle.TypeBottle.Name,
		},
		CreatedAt:   bottle.CreatedAt,
		UpdatedAt:   bottle.UpdatedAt,
	}
	bottleResponseJSON, err := json.Marshal(bottleResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bottle response: %v", err)
	}
	if err := config.RedisClient.Set(ctx, cachedKey, bottleResponseJSON, 15*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("failed to cache bottle response: %v", err)
	}
	return &bottleResponse, nil
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
	bottle.IsNew = input.IsNew
	bottle.IsFavorite = input.IsFavorite
	if err := config.DB.Save(&bottle).Error; err != nil {
		return nil, fmt.Errorf("failed to update bottle: %v", err)
	}
	config.RedisClient.Del(context.Background(), fmt.Sprintf("bottle:%d", id))
	config.RedisClient.Del(context.Background(), "bottles:all")
	if err := config.DB.Preload("TypeBottle").First(&bottle, id).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve updated bottle: %v", err)
	}
	return &models.BottleResponseDTO{
		ID:          bottle.ID,
		Name:        bottle.Name,
		Description: bottle.Description,
		Image:       bottle.Image,
		IsNew:      bottle.IsNew,
		IsFavorite: bottle.IsFavorite,
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
	config.RedisClient.Del(context.Background(), fmt.Sprintf("bottle:%d", id))
	config.RedisClient.Del(context.Background(), "bottles:all")
	return nil
}