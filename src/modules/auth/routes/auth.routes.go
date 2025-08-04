package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/auth/controller"
	"fmt"
	"os"
	"github.com/gin-gonic/gin"
)


func RegisterAuthRoutes(router *gin.RouterGroup) {
	AuthGroup := router.Group("/auth");
	{
		AuthGroup.POST("/register", controller.Register)
		AuthGroup.POST("/login", controller.LoginManual)
		AuthGroup.GET("/google", controller.GoogleLogin)
		AuthGroup.GET("/google/callback", controller.GoogleCallback)
		AuthGroup.POST("/logout", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")), controller.Logout)
		AuthGroup.POST("/refresh", authmiddlewares.RefreshTokenMiddleware(os.Getenv("JWT_REFRESH_SECRET")), controller.RefreshToken)
	}
	fmt.Println("Auth routes registered")
}