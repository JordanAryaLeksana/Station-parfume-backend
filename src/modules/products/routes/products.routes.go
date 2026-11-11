package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/products/controller"
	"backend/src/repository"
	"os"

	"github.com/gin-gonic/gin"
)

// RegisterProductsRoutes: katalog publik (tanpa login) + endpoint admin katalog & stok.
func RegisterProductsRoutes(router *gin.RouterGroup) {
	router.GET("/categories", controller.GetAllCategories)
	router.GET("/sizes", controller.GetSizes)
	router.GET("/products", controller.GetAllProducts) // ?category=&kind=&grade=&note=&q=
	router.GET("/products/:id", controller.GetProduct) // :id = angka atau slug

	auth := authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET"))

	// Produk, harga, ukuran: owner & admin.
	catalog := router.Group("/admin", auth, authmiddlewares.RoleBasedAccess(repository.RoleOwner, repository.RoleAdmin))
	{
		catalog.GET("/products", controller.AdminListProducts) // ?status=
		catalog.POST("/products", controller.AdminCreateProduct)
		catalog.GET("/products/:id", controller.AdminGetProduct)
		catalog.PATCH("/products/:id", controller.AdminUpdateProduct)
		catalog.PATCH("/products/:id/rates", controller.AdminSetProductRates)             // tarif per ml semua wangian
		catalog.PUT("/products/:id/sizes", controller.AdminSetProductSizes)               // ukuran khusus produk
		catalog.POST("/products/:id/scents", controller.AdminAddScent)                    // + kombinasi otomatis
		catalog.POST("/products/:id/variants/generate", controller.AdminGenerateVariants) // "Buat kombinasi"
		catalog.POST("/products/:id/variants", controller.AdminAddGeneralVariant)         // produk umum
		catalog.POST("/products/:id/images", controller.AdminAddImage)                    // url = link CDN
		catalog.PATCH("/images/:id", controller.AdminUpdateImage)
		catalog.DELETE("/images/:id", controller.AdminDeleteImage)
		catalog.PATCH("/scents/:id", controller.AdminUpdateScent)
		catalog.PATCH("/variants", controller.AdminUpdateVariants) // grid harga (banyak sekaligus)
		catalog.GET("/sizes", controller.AdminListSizes)
		catalog.POST("/sizes", controller.AdminCreateSize)
		catalog.PATCH("/sizes/:id", controller.AdminUpdateSize)
	}
	// Stok: owner, admin, gudang.
	inventory := router.Group("/admin/inventory", auth,
		authmiddlewares.RoleBasedAccess(repository.RoleOwner, repository.RoleAdmin, repository.RoleWarehouse))
	{
		inventory.POST("/adjustments", controller.AdminAdjustInventory)
		inventory.GET("/movements", controller.AdminListMovements)
	}
}
