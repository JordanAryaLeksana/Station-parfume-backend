package routes

import (
	authRoutes "backend/src/modules/auth/routes"
	productsRoutes "backend/src/modules/products/routes"
	userRoutes "backend/src/modules/user/routes"
	adminRoutes "backend/src/modules/admin/routes"
	"github.com/gin-gonic/gin"
)


func InitRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	userRoutes.RegisterUserRoutes(api)
	authRoutes.RegisterAuthRoutes(api)
	productsRoutes.RegisterProductsRoutes(api)
	adminRoutes.RegisterAdminRoutes(api)
	api.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Welcome to Station Parfume API!"})
	})
}