package services

import (
	"backend/src/config"
	"backend/src/modules/user/models"
	"backend/src/repository"
	"fmt"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

var validate = validator.New()

func CreateUser(input *models.UserDTORequest) (*models.UserDTOResponse, error) {

	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	var user repository.User

	if err := config.DB.Where("email = ?", input.Email).First(&user).Error; err == nil {
		return nil, fmt.Errorf("user already exists with email: %s", user.Email)
	}

	var newUser = repository.User{
		Name:     input.Name,
		Email:    input.Email,
		Password: input.Password,
		Role:     input.Role,
		Picture:  input.Picture,
		Provider: input.Provider,
		Sub:      input.Sub,
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newUser.Password), bcrypt.MaxCost)
	if err != nil {
		return nil, err
	}
	newUser.Password = string(hashedPassword)

	if err := config.DB.Create(&newUser).Error; err != nil {
		return nil, err
	}

	var userReponse = models.UserDTOResponse{
		Name:     newUser.Name,
		Email:    newUser.Email,
		Role:     newUser.Role,
		Picture:  newUser.Picture,
		Provider: newUser.Provider,
		Sub:      newUser.Sub,
	}

	return &userReponse, nil

}

func GetAllUsers() ([]models.UserDTOResponse, error) {
	var users []repository.User
	if err := config.DB.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve users: %v", err)
	}
	var userResponses []models.UserDTOResponse
	for _, user := range users {
		userResponses = append(userResponses, models.UserDTOResponse{
			Name:     user.Name,
			Email:    user.Email,
			Role:     user.Role,
			Picture:  user.Picture,
			Provider: user.Provider,
			Sub:      user.Sub,
		})
	}
	if len(userResponses) == 0 {
		return nil, fmt.Errorf("no users found")
	}
	return userResponses, nil
}

func GetUserById(id string) (*models.UserDTOResponse, error) {
	var user repository.User

	if err := config.DB.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("user not found with id:%s", id)
	}

	var userResponse = models.UserDTOResponse{
		Name:     user.Name,
		Email:    user.Email,
		Role:     user.Role,
		Picture:  user.Picture,
		Provider: user.Provider,
		Sub:      user.Sub,
	}
	return &userResponse, nil

}

func UpdateUser(id string, request *models.UserDTORequest) (*models.UserDTOResponse, error) {
	if err := validate.Struct(request); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	var user repository.User
	if err := config.DB.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("user not found with id:%s", id)
	}
	var hashedPassword, err = bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.MaxCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}
	if err := config.DB.Model(&user).Updates(request).Error; err != nil {
		return nil, fmt.Errorf("failed to update user: %v", err)
	}
	var updatedUser = repository.User{
		Name:     request.Name,
		Email:    request.Email,
		Password: request.Password,
		Role:     request.Role,
		Picture:  request.Picture,
		Provider: request.Provider,
		Sub:      request.Sub,
	}

	updatedUser.Password = string(hashedPassword)
	var userReponse = models.UserDTOResponse{
		Name:     updatedUser.Name,
		Email:    updatedUser.Email,
		Role:     updatedUser.Role,
		Picture:  updatedUser.Picture,
		Provider: updatedUser.Provider,
		Sub:      updatedUser.Sub,
	}

	return &userReponse, nil
}

func DeleteUser(id string) error {
	var user repository.User
	if err := config.DB.First(&user, id).Error; err != nil {
		return fmt.Errorf("user not found with id: %s", id)
	}

	if err := config.DB.Delete(&user).Error; err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}
	return nil
}
