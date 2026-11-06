package routes

import (
	adminRoutes "backend/src/modules/admin/routes"
	authRoutes "backend/src/modules/auth/routes"
	cartRoutes "backend/src/modules/cart/routes"
	paymentRoutes "backend/src/modules/payment/routes"
	productsRoutes "backend/src/modules/products/routes"
	userRoutes "backend/src/modules/user/routes"
	"github.com/gin-gonic/gin"
)

func InitRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	userRoutes.RegisterUserRoutes(api)
	authRoutes.RegisterAuthRoutes(api)
	productsRoutes.RegisterProductsRoutes(api)
	adminRoutes.RegisterAdminRoutes(api)
	cartRoutes.RegisterCartRoutes(api)
	paymentRoutes.RegisterPaymentRoutes(api)
	api.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Welcome to Station Parfume API!"})
	})
}
