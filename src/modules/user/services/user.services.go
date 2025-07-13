package services

import (
	"backend/src/config"
	"backend/src/modules/user/models"
	"backend/src/repository"
	"fmt"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"time"
	
)

var validate = validator.New()

func CreateUser(input *models.UserDTORequest) (*models.UserDTOResponse, error) {
	fmt.Println("✅ Memulai service CreateUser")
	fmt.Println("📧 Email input:", input.Email)

	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	fmt.Println("🔍 Validasi input berhasil");
	fmt.Println(input.Password)
	start := time.Now()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	fmt.Printf("🕒 Hashing selesai dalam %v\n", time.Since(start))

	if err != nil {
		fmt.Println("❌ Error hashing password:", err)
		return nil, err
	}
	fmt.Println("🔐 Password hashed successfully")
	newUser := repository.User{
		Name:     input.Name,
		Email:    input.Email,
		Password: string(hashedPassword),
		// Password: input.Password,
		Role:     input.Role,
		Picture:  input.Picture,
		Sub: nil,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		return nil, err
	}

	userResponse := models.UserDTOResponse{
		ID:       newUser.ID,
		Name:     newUser.Name,
		Email:    newUser.Email,
		Role:     newUser.Role,
		Picture:  newUser.Picture,
		Provider: newUser.Provider,
		Sub:      newUser.Sub,
	}

	return &userResponse, nil
}

func GetAllUsers() ([]models.UserDTOResponse, error) {
	var users []repository.User
	if err := config.DB.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve users: %v", err)
	}
	var userResponses []models.UserDTOResponse
	for _, user := range users {
		userResponses = append(userResponses, models.UserDTOResponse{
			ID: 	 user.ID,
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
		ID:       user.ID,
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
	var hashedPassword, err = bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
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
	}

	updatedUser.Password = string(hashedPassword)
	var userReponse = models.UserDTOResponse{
		ID:       updatedUser.ID,
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
