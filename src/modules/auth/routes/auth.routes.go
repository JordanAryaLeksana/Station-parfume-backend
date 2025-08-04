package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/auth/controller"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)


func AuthRoutes(router *gin.RouterGroup) {
	registerAuthGroup := router.Group("/auth");
	{
		registerAuthGroup.POST("/register", controller.Register)
		registerAuthGroup.POST("/login", controller.LoginManual)
		registerAuthGroup.GET("/google", controller.GoogleLogin)
		registerAuthGroup.GET("/google/callback", controller.GoogleCallback)
		registerAuthGroup.POST("/logout", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")), controller.Logout)
		registerAuthGroup.POST("/refresh", authmiddlewares.RefreshTokenMiddleware(os.Getenv("JWT_REFRESH_SECRET")), controller.RefreshToken)
	}
	fmt.Println("Auth routes registered")
}