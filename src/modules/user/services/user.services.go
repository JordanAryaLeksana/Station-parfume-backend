package services

import (
	"backend/src/config"
	"backend/src/modules/user/models"
	"backend/src/repository"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

var validate = validator.New()

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
	ErrForbidden  = errors.New("forbidden")
)

// Aturan peran di /users (dipanggil owner atau admin):
//   - admin hanya boleh mengelola pembeli (role "user");
//   - owner boleh juga mengelola staf (admin, warehouse, cs);
//   - akun owner tidak pernah bisa dibuat/diubah/dihapus lewat endpoint ini
//     (owner pertama dibuat lewat `go run ./seed -admin`).
func canManageRole(caller, role string) bool {
	if role == repository.RoleUser {
		return caller == repository.RoleOwner || caller == repository.RoleAdmin
	}
	return caller == repository.RoleOwner && role != repository.RoleOwner && slices.Contains(repository.StaffRoles, role)
}

func checkRole(caller, role string) error {
	if role != repository.RoleUser && !slices.Contains(repository.StaffRoles, role) {
		return fmt.Errorf("%w: role %q tidak dikenal", ErrValidation, role)
	}
	if !canManageRole(caller, role) {
		return fmt.Errorf("%w: role %s tidak boleh mengelola akun ber-role %s", ErrForbidden, caller, role)
	}
	return nil
}

func CreateUser(callerRole string, input *models.UserDTORequest) (*models.UserDTOResponse, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
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
	if err := checkRole(callerRole, role); err != nil {
		return nil, err
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

func UpdateUser(callerRole, id string, request *models.UserDTORequest) (*models.UserDTOResponse, error) {
	if err := validate.Struct(request); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	user, err := findUser(id)
	if err != nil {
		return nil, err
	}
	// Akun yang diubah harus boleh dikelola pemanggil, begitu juga role barunya.
	if err := checkRole(callerRole, user.Role); err != nil {
		return nil, err
	}
	if request.Role != "" {
		if err := checkRole(callerRole, request.Role); err != nil {
			return nil, err
		}
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

func DeleteUser(callerRole, id string) error {
	user, err := findUser(id)
	if err != nil {
		return err
	}
	if err := checkRole(callerRole, user.Role); err != nil {
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
		return nil, fmt.Errorf("user %s: %w", id, ErrNotFound)
	}
	var user repository.User
	if err := config.DB.First(&user, uid).Error; err != nil {
		return nil, fmt.Errorf("user %s: %w", id, ErrNotFound)
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
