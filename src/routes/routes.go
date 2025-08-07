package routes

import (
	"github.com/gin-gonic/gin"
	userRoutes "backend/src/modules/user/routes"
	authRoutes "backend/src/modules/auth/routes"
	parfumeRoutes "backend/src/modules/parfume/routes"
)


func InitRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	userRoutes.RegisterUserRoutes(api)
	authRoutes.RegisterAuthRoutes(api)
	parfumeRoutes.RegisterParfumeRoutes(api)
	api.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Welcome to Station Parfume API!"})
	})
}