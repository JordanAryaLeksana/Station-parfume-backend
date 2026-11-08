package routes

import (
	"backend/src/modules/products/controller"

	"github.com/gin-gonic/gin"
)

// RegisterProductsRoutes: katalog yang meniru toko Shopee (kategori + listing).
// Semua publik (pengunjung bisa lihat produk tanpa login, seperti di Shopee) dan
// read-only: isinya disamakan dengan Shopee lewat seeder (seed/categories.csv,
// seed/catalog.csv), jadi tidak ada endpoint tulis di sini.
func RegisterProductsRoutes(router *gin.RouterGroup) {
	router.GET("/categories", controller.GetAllCategories)

	productGroup := router.Group("/products")
	{
		productGroup.GET("/", controller.GetAllProducts) // ?category=<slug kategori>
		productGroup.GET("/:id", controller.GetProductByID)
	}
}
