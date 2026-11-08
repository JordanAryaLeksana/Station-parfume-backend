package routes

import (
	"backend/src/modules/products/controller"

	"github.com/gin-gonic/gin"
)

// RegisterProductsRoutes: katalog (kategori + listing seperti Shopee) dan wangian dari foto produk.
// Semua publik (pengunjung bisa lihat produk tanpa login, seperti di Shopee) dan
// read-only: isinya diisi lewat seeder (seed/categories.csv, seed/wangian.csv,
// seed/catalog.csv), jadi tidak ada endpoint tulis di sini.
func RegisterProductsRoutes(router *gin.RouterGroup) {
	router.GET("/categories", controller.GetAllCategories)
	router.GET("/scents", controller.GetScentGroups) // ?grade=regular|exclusive&note=woody

	productGroup := router.Group("/products")
	{
		productGroup.GET("/", controller.GetAllProducts) // ?category=<slug kategori>
		productGroup.GET("/:id", controller.GetProductByID)
	}
}
