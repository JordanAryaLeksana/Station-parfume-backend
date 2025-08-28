package services

import (
	"backend/src/config"
	"backend/src/modules/auth/models"
	"backend/src/repository"
	authhandler "backend/src/utils/AuthHandler"
	"backend/src/utils/jwt"
	utils "backend/src/utils/redisUtils"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"strings"
	"time"
	// "google.golang.org/api/idtoken"
)

var validate = validator.New()

func Register(input *models.RegisterRequest) (*models.RegisterResponse, error) {
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

func LoginManual(input *models.LoginRequest) (*models.LoginResponse, error) {
	var user repository.User
	err := config.DB.Where("email = ?", input.Email).First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}
	if user.Provider != "local" {
		return nil, fmt.Errorf("please login using %s", user.Provider)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return nil, fmt.Errorf("invalid password: %v", err)
	}

	access_token, err := jwt.GenerateAccessToken(user)
	if err != nil || access_token == "" {
		return nil, fmt.Errorf("error generating access token: %v", err)
	}
	refresh_token, err := jwt.GenerateRefreshToken(user)
	if err != nil || refresh_token == "" {
		return nil, fmt.Errorf("error generating refresh token: %v", err)
	}
	return &models.LoginResponse{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		Role:         user.Role,
		Picture:      user.Picture,
		Provider:     user.Provider,
		Sub:          user.Sub,
		AccessToken:  access_token,
		RefreshToken: refresh_token,
	}, nil
}
func LoginAdmin(input *models.LoginRequest) (*models.LoginResponse, error) {
	var user repository.User
	err := config.DB.Where("email = ?", input.Email).First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Validasi role khusus admin
	if user.Role != "admin" {
		return nil, fmt.Errorf("only admin can login here")
	}

	if user.Provider != "local" {
		return nil, fmt.Errorf("please login using %s", user.Provider)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return nil, fmt.Errorf("invalid password: %v", err)
	}

	access_token, err := jwt.GenerateAccessToken(user)
	if err != nil || access_token == "" {
		return nil, fmt.Errorf("error generating access token: %v", err)
	}

	refresh_token, err := jwt.GenerateRefreshToken(user)
	if err != nil || refresh_token == "" {
		return nil, fmt.Errorf("error generating refresh token: %v", err)
	}

	return &models.LoginResponse{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		Role:         user.Role,
		Picture:      user.Picture,
		Provider:     user.Provider,
		Sub:          user.Sub,
		AccessToken:  access_token,
		RefreshToken: refresh_token,
	}, nil
}

func HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := authhandler.GoogleOauthConfig.AuthCodeURL("random-state-string", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
func HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No code in request", http.StatusBadRequest)
		return
	}

	if authhandler.GoogleOauthConfig == nil {
		http.Error(w, "Google OAuth config not set", http.StatusInternalServerError)
		return
	}
	// Swap the code for a token
	token, err := authhandler.GoogleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Code exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if token == nil || token.AccessToken == "" {
		http.Error(w, "Failed to get access token", http.StatusInternalServerError)
		return
	}
	// Get user info from Google
	client := authhandler.GoogleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "Failed to get userinfo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo models.AuthGoogleResponse
	json.NewDecoder(resp.Body).Decode(&userInfo)

	// direct into the database
	var user repository.User
	err = config.DB.Where("email = ?", userInfo.Email).First(&user).Error
	if err == nil {
		if user.Provider != "google" {
			http.Error(w, "Email already registered with a different provider", http.StatusConflict)
			return
		}
	} else {
		user = repository.User{
			Name:     userInfo.Name,
			Email:    userInfo.Email,
			Picture:  userInfo.Picture,
			Sub:      userInfo.Sub,
			Provider: "google",
			Role:     "user", // default role
		}
		if err := config.DB.Create(&user).Error; err != nil {
			http.Error(w, "DB Error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// Generate JWT
	access_token, err := jwt.GenerateAccessToken(user)
	if err != nil || access_token == "" {
		http.Error(w, "JWT Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	refresh_token, err := jwt.GenerateRefreshToken(user)
	if err != nil || refresh_token == "" {
		http.Error(w, "JWT Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "http://localhost:3000/oauth-success?access_token="+access_token+"&refresh_token="+refresh_token, http.StatusSeeOther)
}

func Logout(authHeader string) error {
	if authHeader == "" {
		return fmt.Errorf("authorization header is empty")
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == authHeader {
		return fmt.Errorf("invalid authorization header format")
	}

	claims, err := jwt.ParseRefreshToken(tokenStr)
	if err != nil {
		return fmt.Errorf("error parsing refresh token: %v", err)
	}

	jti := claims.ID
	if jti == "" {
		return fmt.Errorf("token ID (jti) is empty")
	}

	exp := claims.ExpiresAt.Time
	if exp.IsZero() {
		return fmt.Errorf("token expiration time is not set")
	}
	ttl := time.Until(time.Unix(exp.Unix(), 0))
	if ttl <= 0 {
		return fmt.Errorf("token has already expired")
	}
	err = utils.BlacklistJti(jti, ttl)
	log.Printf("Blacklisting token with jti: %s, ttl: %v", jti, ttl)
	if err != nil {
		return fmt.Errorf("error blacklisting token: %v", err)
	}
	return nil
}

func RefreshToken(authHeader string) (*models.PairToken, error) {
	if authHeader == "" {
		return nil, fmt.Errorf("authorization header is empty")
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == authHeader {
		return nil, fmt.Errorf("invalid authorization header format")
	}
	claims, err := jwt.ParseRefreshToken(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing refresh token: %v", err)
	}
	isBlacklisted, err := utils.IsJtiBlacklisted(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check blacklist: %v", err)
	}
	if isBlacklisted {
		return nil, fmt.Errorf("refresh token is blacklisted")
	}
	var user repository.User
	if err := config.DB.First(&user, claims.UserID).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Generate new tokens
	accessToken, err := jwt.GenerateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %v", err)
	}
	refreshToken, err := jwt.GenerateRefreshToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %v", err)
	}

	jti := claims.ID
	if jti == "" {
		return nil, fmt.Errorf("token ID (jti) is empty")
	}
	exp := claims.ExpiresAt.Time
	if exp.IsZero() {
		return nil, fmt.Errorf("token expiration time is not set")
	}
	ttl := time.Until(exp)
	if ttl > 0 {
		if err := utils.BlacklistJti(jti, ttl); err != nil {
			return nil, fmt.Errorf("error blacklisting old token: %v", err)
		}
	}
	return &models.PairToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
