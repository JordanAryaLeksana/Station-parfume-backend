package routes

import (
	"backend/src/modules/user/controller"
	"fmt"
	"github.com/gin-gonic/gin"
)


func AuthRoutes(router *gin.RouterGroup) {
	registerUserGroup := router.Group("/auth");
	{
		registerUserGroup.POST("/register", controller.Register)
		registerUserGroup.POST("/login", controller.Login)
		registerUserGroup.GET("/google", controller.GoogleLogin)
		registerUserGroup.GET("/google/callback", controller.GoogleCallback)
		registerUserGroup.POST("/logout", controller.Logout)
	}
	fmt.Println("Auth routes registered")
}