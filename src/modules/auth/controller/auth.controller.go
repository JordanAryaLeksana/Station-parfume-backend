package controller

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/auth/models"
	"backend/src/modules/auth/services"
	"backend/src/utils/jwt"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// writeError: pesan error internal tidak pernah dikirim ke klien (hanya ke log).
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrInvalidCredentials):
		httperror.UnauthorizedError(c, services.ErrInvalidCredentials.Error())
	case errors.Is(err, services.ErrUnauthorized):
		httperror.UnauthorizedError(c, "Sesi tidak valid, silakan login ulang")
	case errors.Is(err, services.ErrConflict):
		httperror.ConflictError(c, services.ErrConflict.Error())
	case errors.Is(err, services.ErrValidation):
		httperror.BadRequestError(c, err.Error())
	case errors.Is(err, services.ErrUnavailable):
		log.Printf("auth: %v", err)
		httperror.ServiceUnavailableError(c, "Layanan autentikasi sedang bermasalah, coba lagi")
	default:
		log.Printf("auth: %v", err)
		httperror.InternalServerError(c, "Internal server error")
	}
}

func Register(c *gin.Context) {
	var input models.RegisterRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	user, err := services.Register(&input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "User Registered", "user": user})
}

func LoginManual(c *gin.Context) {
	var input models.LoginRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		// format salah (mis. password < 8) → pesan yang sama dengan kredensial salah
		httperror.UnauthorizedError(c, services.ErrInvalidCredentials.Error())
		return
	}
	user, err := services.LoginManual(&input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User Logged In", "user": user})
}

func LoginAdmin(c *gin.Context) {
	var input models.LoginRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.UnauthorizedError(c, services.ErrInvalidCredentials.Error())
		return
	}
	user, err := services.LoginAdmin(&input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Admin Logged In", "user": user})
}

func GoogleLogin(c *gin.Context)    { services.HandleGoogleLogin(c) }
func GoogleCallback(c *gin.Context) { services.HandleGoogleCallback(c) }

// Logout: header Authorization = access token; body opsional {"refresh_token": "..."}.
func Logout(c *gin.Context) {
	var input models.LogoutRequest
	_ = c.ShouldBindJSON(&input) // body opsional
	exp, _ := c.Get(authmiddlewares.CtxExpiresAt)
	expTime, _ := exp.(time.Time)
	err := services.Logout(c.GetUint(authmiddlewares.CtxUserID), c.GetString(authmiddlewares.CtxJTI), expTime, input.RefreshToken)
	if err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RefreshToken: header Authorization = refresh token (diverifikasi RefreshTokenMiddleware).
func RefreshToken(c *gin.Context) {
	v, _ := c.Get(authmiddlewares.CtxRefreshClaims)
	claims, ok := v.(*jwt.JwtCustomClaims)
	if !ok {
		httperror.UnauthorizedError(c, "Unauthorized")
		return
	}
	tokens, err := services.RefreshToken(claims)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Token Refreshed", "tokens": tokens})
}
