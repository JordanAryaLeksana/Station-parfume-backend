package services

import (
	"backend/src/config"
	"backend/src/modules/admin/models"
	"backend/src/repository"
	"strconv"

	"fmt"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

var validate = validator.New()

func CreateAdmin(input *models.AdminRequest) (*models.AdminResponse, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %v", err)
	}
	if input.Email == "" {
		return nil, fmt.Errorf("invalid email")
	}
	var user repository.User
	if err := config.DB.Where("lower(email) = ?", repository.NormalizeEmail(input.Email)).First(&user).Error; err == nil {
		return nil, fmt.Errorf("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %v", err)
	}
	hash := string(hashedPassword)
	createAdmin := repository.User{
		Name:         input.Name,
		Email:        repository.NormalizeEmail(input.Email),
		PasswordHash: &hash,
		Provider:     repository.ProviderLocal,
		Role:         repository.RoleAdmin,
	}
	if err := config.DB.Create(&createAdmin).Error; err != nil {
		return nil, fmt.Errorf("failed to create admin: %v", err)
	}
	return &models.AdminResponse{
		Email: createAdmin.Email,
		Role:  createAdmin.Role,
	}, nil
}

func GetAdminByID(id string) (*models.AdminResponse, error) {
	admin, err := findAdmin(id)
	if err != nil {
		return nil, err
	}
	return &models.AdminResponse{
		Email: admin.Email,
		Role:  admin.Role,
	}, nil
}

func UpdateAdmin(id string, input *models.AdminRequest) (*models.AdminResponse, error) {
	admin, err := findAdmin(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if input.Email != "" {
		updates["email"] = repository.NormalizeEmail(input.Email)
	}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("error hashing password: %v", err)
		}
		updates["password_hash"] = string(hashedPassword)
	}

	if len(updates) > 0 {
		if err := config.DB.Model(admin).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update admin: %v", err)
		}
	}

	return &models.AdminResponse{
		Email: admin.Email,
		Role:  admin.Role,
	}, nil
}

func DeleteAdmin(id string) error {
	admin, err := findAdmin(id)
	if err != nil {
		return err
	}
	if err := config.DB.Delete(admin).Error; err != nil {
		return fmt.Errorf("failed to delete admin: %v", err)
	}
	return nil
}

// findAdmin memvalidasi id sebagai angka dulu supaya string dari URL tidak
// pernah masuk ke query sebagai SQL mentah.
func findAdmin(id string) (*repository.User, error) {
	uid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("admin not found with id:%s", id)
	}
	// Hanya baris ber-role admin: /admins/:id tidak boleh menyentuh user biasa.
	var admin repository.User
	if err := config.DB.Where("role = ?", repository.RoleAdmin).First(&admin, uid).Error; err != nil {
		return nil, fmt.Errorf("admin not found with id:%s", id)
	}
	return &admin, nil
}
