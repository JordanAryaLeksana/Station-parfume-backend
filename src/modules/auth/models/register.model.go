package models

// RegisterRequest: password 8–72 karakter (72 byte = batas bcrypt).
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,max=255"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type RegisterResponse struct {
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	Sub      *string `json:"sub"`
	Provider string  `json:"provider"`
}
