package controller

import (
	"backend/src/middlewares/Error"
	"backend/src/modules/auth/models"
	"backend/src/modules/auth/services"
	"github.com/gin-gonic/gin"
)


func Register(c *gin.Context) {
	var input models.RegisterRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	user, err := services.Register(&input)
	if err != nil {
		httperror.BadRequestError(c, err.Error())
		return
	}
	c.JSON(201, user)
}

func LoginManual(c *gin.Context) {
	var input models.LoginRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	user, err := services.LoginManual(&input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, user)
}

func GoogleLogin(c *gin.Context) {
	services.HandleGoogleLogin(c.Writer, c.Request)
}

func GoogleCallback(c *gin.Context) {
	services.HandleGoogleCallback(c.Writer, c.Request)
}


func Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		httperror.UnauthorizedError(c, "Authorization header is missing")
		return
	}
	if err := services.Logout(authHeader); err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(204, nil)
}

func RefreshToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		httperror.UnauthorizedError(c, "Authorization header is missing")
		return
	}
	newTokens, err := services.RefreshToken(authHeader);
		if err != nil {
		httperror.UnauthorizedError(c, err.Error())
		return
	}
	c.JSON(200, newTokens)
}	