package routes

import (
	"backend/src/modules/auth/controller"
	"fmt"
	"github.com/gin-gonic/gin"
)


func AuthRoutes(router *gin.RouterGroup) {
	registerAuthGroup := router.Group("/auth");
	{
		registerAuthGroup.POST("/register", controller.Register)
		registerAuthGroup.POST("/login", controller.LoginManual)
		registerAuthGroup.GET("/google", controller.GoogleLogin)
		registerAuthGroup.GET("/google/callback", controller.GoogleCallback)
		registerAuthGroup.POST("/logout", controller.Logout)
		registerAuthGroup.POST("/refresh", controller.RefreshToken)
	}
	fmt.Println("Auth routes registered")
}