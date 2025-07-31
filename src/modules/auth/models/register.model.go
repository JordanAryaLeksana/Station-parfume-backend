package models

type RegisterRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password,omitempty"` // optional for OAuth    
}

type RegisterResponse struct {
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	Sub      *string `json:"sub"`
	Provider string  `json:"provider"`
}
