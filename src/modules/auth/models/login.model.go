package models

// GoogleUserInfo = respons userinfo OpenID Connect Google.
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// LogoutRequest: refresh_token opsional; kalau dikirim ikut dicabut.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LoginResponse struct {
	ID           uint    `json:"id"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	Role         string  `json:"role"`
	Picture      string  `json:"picture"`
	Provider     string  `json:"provider"`
	Sub          *string `json:"sub"`
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
}

type PairToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
