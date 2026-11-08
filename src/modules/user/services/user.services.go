package services

import (
	"backend/src/config"
	"backend/src/modules/user/models"
	"backend/src/repository"
	"fmt"
	"strconv"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

var validate = validator.New()

func CreateUser(input *models.UserDTORequest) (*models.UserDTOResponse, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	hash := string(hashedPassword)
	role := input.Role
	if role == "" {
		role = repository.RoleUser
	}
	newUser := repository.User{
		Name:         input.Name,
		Email:        repository.NormalizeEmail(input.Email),
		PasswordHash: &hash,
		Role:         role,
		PictureURL:   optionalString(input.Picture),
		Provider:     repository.ProviderLocal,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		return nil, err
	}
	resp := toUserResponse(newUser)
	return &resp, nil
}

func GetAllUsers() ([]models.UserDTOResponse, error) {
	var users []repository.User
	if err := config.DB.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve users: %v", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("no users found")
	}
	userResponses := make([]models.UserDTOResponse, 0, len(users))
	for _, user := range users {
		userResponses = append(userResponses, toUserResponse(user))
	}
	return userResponses, nil
}

func GetUserById(id string) (*models.UserDTOResponse, error) {
	user, err := findUser(id)
	if err != nil {
		return nil, err
	}
	resp := toUserResponse(*user)
	return &resp, nil
}

func UpdateUser(id string, request *models.UserDTORequest) (*models.UserDTOResponse, error) {
	if err := validate.Struct(request); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	user, err := findUser(id)
	if err != nil {
		return nil, err
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}
	updates := map[string]any{
		"name":          request.Name,
		"email":         repository.NormalizeEmail(request.Email),
		"password_hash": string(hashedPassword),
		"picture_url":   optionalString(request.Picture),
	}
	if request.Role != "" {
		updates["role"] = request.Role
	}
	if err := config.DB.Model(user).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update user: %v", err)
	}
	if err := config.DB.First(user, user.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload user: %v", err)
	}
	resp := toUserResponse(*user)
	return &resp, nil
}

func DeleteUser(id string) error {
	user, err := findUser(id)
	if err != nil {
		return err
	}
	if err := config.DB.Delete(user).Error; err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}
	return nil
}

// findUser memvalidasi id sebagai angka dulu supaya string dari URL tidak
// pernah masuk ke query sebagai SQL mentah.
func findUser(id string) (*repository.User, error) {
	uid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("user not found with id:%s", id)
	}
	var user repository.User
	if err := config.DB.First(&user, uid).Error; err != nil {
		return nil, fmt.Errorf("user not found with id:%s", id)
	}
	return &user, nil
}

func toUserResponse(u repository.User) models.UserDTOResponse {
	picture := ""
	if u.PictureURL != nil {
		picture = *u.PictureURL
	}
	return models.UserDTOResponse{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Role:     u.Role,
		Picture:  picture,
		Provider: u.Provider,
		Sub:      u.ProviderSub,
	}
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
