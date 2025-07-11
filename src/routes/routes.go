package routes

import (
	"github.com/gin-gonic/gin"
	"backend/src/modules/user/routes"
)


func InitRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	routes.RegisterUserRoutes(api)
	api.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Welcome to Station Parfume API!"})
	})
}