// Package jwt: pembuatan & verifikasi access/refresh token (HS256).
// Semua parsing lewat ParseAccessToken / ParseRefreshToken supaya aturan
// (algoritma, issuer, tipe token, jti, exp) sama di semua tempat.
package jwt

import (
	"backend/src/repository"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	Issuer      = "station-parfume"
	TypeAccess  = "access"
	TypeRefresh = "refresh"

	AccessTTL  = 15 * time.Minute
	RefreshTTL = 24 * time.Hour
)

var ErrInvalidToken = errors.New("invalid token")

type JwtCustomClaims struct {
	UserID    uint   `json:"sub"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(user repository.User) (string, error) {
	return sign(user, TypeAccess, AccessTTL, os.Getenv("JWT_SECRET"))
}

func GenerateRefreshToken(user repository.User) (string, error) {
	return sign(user, TypeRefresh, RefreshTTL, os.Getenv("JWT_REFRESH_SECRET"))
}

func sign(user repository.User, typ string, ttl time.Duration, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret kosong")
	}
	now := time.Now()
	claims := JwtCustomClaims{
		UserID: user.ID, Email: user.Email, Role: user.Role, TokenType: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.NewString(), // jti: dipakai untuk blacklist/logout
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseAccessToken memverifikasi access token dengan secret yang diberikan.
func ParseAccessToken(tokenStr, secret string) (*JwtCustomClaims, error) {
	return parse(tokenStr, secret, TypeAccess)
}

// ParseRefreshToken memverifikasi refresh token dengan secret yang diberikan.
func ParseRefreshToken(tokenStr, secret string) (*JwtCustomClaims, error) {
	return parse(tokenStr, secret, TypeRefresh)
}

func parse(tokenStr, secret, typ string) (*JwtCustomClaims, error) {
	if secret == "" {
		return nil, fmt.Errorf("%w: secret kosong", ErrInvalidToken)
	}
	claims := &JwtCustomClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(*jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.TokenType != typ || claims.ID == "" || claims.UserID == 0 {
		return nil, fmt.Errorf("%w: tipe/jti/sub tidak valid", ErrInvalidToken)
	}
	return claims, nil
}
