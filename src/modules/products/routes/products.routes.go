package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/products/controller"
	"os"

	"github.com/gin-gonic/gin"
)

// RegisterProductsRoutes: katalog (listing Shopee) + master data aroma, brand, botol.
// Listing (products/models/variasi) diisi lewat seeder dari seed/catalog.csv,
// jadi di API hanya bisa dibaca. Middleware role ditaruh SEBELUM handler.
func RegisterProductsRoutes(router *gin.RouterGroup) {
	auth := authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET"))
	admin := authmiddlewares.RoleBasedAccess("admin")

	productGroup := router.Group("/products", auth)
	{
		productGroup.GET("/", controller.GetAllProducts) // ?type=perfume|empty_bottle|other
		productGroup.GET("/:id", controller.GetProductByID)
	}
	aromaGroup := router.Group("/aromas", auth)
	{
		aromaGroup.GET("/", controller.GetAllAromas)
		aromaGroup.GET("/:id", controller.GetAromaByID)
		aromaGroup.PATCH("/:id", admin, controller.UpdateAroma)
	}
	brandGroup := router.Group("/brands", auth)
	{
		brandGroup.GET("/", controller.GetAllBrands)
		brandGroup.POST("/", admin, controller.CreateBrand)
	}
	bottleGroup := router.Group("/bottles", auth)
	{
		bottleGroup.GET("/", controller.GetAllBottles)
		bottleGroup.GET("/:id", controller.GetBottleByID)
		bottleGroup.POST("/", admin, controller.CreateBottle)
		bottleGroup.PUT("/:id", admin, controller.UpdateBottle)
	}
}
