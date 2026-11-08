package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/products/controller"
	"os"

	"github.com/gin-gonic/gin"
)

// RegisterProductsRoutes: katalog (listing Shopee) + master data aroma, brand, botol.
// Semua GET publik (pengunjung bisa lihat produk tanpa login, seperti di Shopee).
// Endpoint tulis hanya untuk admin: auth + role dipasang SEBELUM handler.
// Listing (products/models/variasi) diisi lewat seeder dari seed/catalog.csv.
func RegisterProductsRoutes(router *gin.RouterGroup) {
	admin := []gin.HandlerFunc{
		authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")),
		authmiddlewares.RoleBasedAccess("admin"),
	}
	with := func(h gin.HandlerFunc) []gin.HandlerFunc {
		return append(append([]gin.HandlerFunc{}, admin...), h)
	}

	productGroup := router.Group("/products")
	{
		productGroup.GET("/", controller.GetAllProducts) // ?type=perfume|empty_bottle|other
		productGroup.GET("/:id", controller.GetProductByID)
	}
	aromaGroup := router.Group("/aromas")
	{
		aromaGroup.GET("/", controller.GetAllAromas)
		aromaGroup.GET("/:id", controller.GetAromaByID)
		aromaGroup.PATCH("/:id", with(controller.UpdateAroma)...)
	}
	brandGroup := router.Group("/brands")
	{
		brandGroup.GET("/", controller.GetAllBrands)
		brandGroup.POST("/", with(controller.CreateBrand)...)
	}
	bottleGroup := router.Group("/bottles")
	{
		bottleGroup.GET("/", controller.GetAllBottles)
		bottleGroup.GET("/:id", controller.GetBottleByID)
		bottleGroup.POST("/", with(controller.CreateBottle)...)
		bottleGroup.PUT("/:id", with(controller.UpdateBottle)...)
	}
}
