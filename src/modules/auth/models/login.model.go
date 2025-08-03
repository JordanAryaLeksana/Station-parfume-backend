package models


type AuthGoogleRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

type AuthGoogleResponse struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	Picture  string  `json:"picture"`
	Provider string  `json:"provider"`
	Sub      *string `json:"sub"`
	Token    string  `json:"token"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	Picture  string  `json:"picture"`
	Provider string  `json:"provider"`
	Sub      *string `json:"sub"`
	AccessToken   string  `json:"access_token"`
	RefreshToken  string  `json:"refresh_token"`
}

type PairToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}