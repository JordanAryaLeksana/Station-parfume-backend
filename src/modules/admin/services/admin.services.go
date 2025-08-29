package services

import (
	"backend/src/config"
	"backend/src/modules/admin/models"
	"backend/src/repository"

	"fmt"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

var validate = validator.New()
func CreateAdmin(input *models.AdminRequest) (*models.AdminResponse, error ) {
	if err := validate.Struct(input); err !=  nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	var user repository.User
	if err := config.DB.Where("email = ?", input.Email).First(&user).Error; err == nil {
		return nil, fmt.Errorf("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %v", err)
	}
	if input.Email != "" {
		createAdmin := repository.User{
			Name:     input.Name,
			Email:    input.Email,
			Password: string(hashedPassword),
			Role:     "admin",
		}
		if err := config.DB.Create(&createAdmin).Error; err != nil {
			return nil, fmt.Errorf("failed to create admin: %v", err)
		}
		return &models.AdminResponse{
			Email:    createAdmin.Email,
			Role:     createAdmin.Role,
		}, nil
	}
	return nil, fmt.Errorf("invalid email")
}

func GetAdminByID(id string ) (*models.AdminResponse, error){
	var admin = repository.User{}
	if err := config.DB.First(&admin, id).Error; err != nil {
		return nil, fmt.Errorf("admin not found with id:%s", id)
	}
	return &models.AdminResponse{
		Email: admin.Email,
		Role:  admin.Role,
	}, nil
}

func UpdateAdmin(id string, input *models.AdminRequest) (*models.AdminResponse, error) {
	var admin repository.User
	if err := config.DB.First(&admin, id).Error; err != nil {
		return nil, fmt.Errorf("admin not found with id:%s", id)
	}

    updateData := repository.User{}

    if input.Email != "" {
        updateData.Email = input.Email
    }
    if input.Name != "" {
        updateData.Name = input.Name
    }
    if input.Password != "" {
        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
        if err != nil {
            return nil, fmt.Errorf("error hashing password: %v", err)
        }
        updateData.Password = string(hashedPassword)
    }

    if updateData != (repository.User{}) {
        if err := config.DB.Model(&admin).Updates(updateData).Error; err != nil {
            return nil, fmt.Errorf("failed to update admin: %v", err)
        }
    }

    return &models.AdminResponse{
        Email: admin.Email,
        Role:  admin.Role,
    }, nil
}


func DeleteAdmin(id string) error {
	var admin repository.User
	if err := config.DB.First(&admin, id).Error; err != nil {
		return fmt.Errorf("admin not found with id:%s", id)
	}
	if err := config.DB.Delete(&admin, id).Error; err != nil {
		return fmt.Errorf("failed to delete admin: %v", err)
	}
	return nil
}