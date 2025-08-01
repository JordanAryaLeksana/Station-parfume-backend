package services

import (
	"backend/src/config"
	"backend/src/modules/auth/models"
	"backend/src/repository"
	"fmt"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"

)

var validate = validator.New()
func Register(input *models.RegisterRequest) (*models.RegisterResponse, error){
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}

	var user repository.User
	err := config.DB.Where("email = ?", input.Email).First(&user).Error
	if err == nil {
		return nil, fmt.Errorf("user with email %s already exists", input.Email)
	}

	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if hashErr != nil {
		return nil, fmt.Errorf("error hashing password: %v", hashErr)
	}
	newUser := repository.User{
		Name:     input.Name,
		Email:    input.Email,
		Password: string(hashedPassword),
		Role:     "user", 
		Picture:  "",
		Sub:      nil,
		Provider: "local", 
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		return nil, fmt.Errorf("error creating user: %v", err)
	}

	userResponse := models.RegisterResponse{
		Name:     newUser.Name,
		Email:    newUser.Email,
		Role:     newUser.Role,
		Sub:      newUser.Sub,
		Provider: newUser.Provider,
	}
	return &userResponse, nil
}

func Login(input *models.LoginRequest) (*models.LoginResponse, error){
	var user repository.User
	err := config.DB.Where("email = ?", input.Email).First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return nil, fmt.Errorf("invalid password: %v", err)
	}

}

func LoginWithGooge() {
	
}