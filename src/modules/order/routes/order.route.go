package routes

import (
	authMiddleware "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/order/controller"
	"github.com/gin-gonic/gin"
	"os"
)

// RegisterOrderRoutes BELUM dipanggil di src/routes/routes.go (sama seperti sebelumnya).
// Daftarkan setelah M3 (middleware auth memanggil Abort), supaya endpoint admin
// /orders/:id/status tidak bisa diakses tanpa token yang valid.
func RegisterOrderRoutes(rg *gin.RouterGroup) {
	orderGroup := rg.Group("/orders", authMiddleware.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		orderGroup.POST("/", controller.CheckoutHandler) // checkout dari keranjang
		orderGroup.GET("/", controller.GetMyOrdersHandler)
		orderGroup.GET("/:id", controller.GetOrderByIDHandler)
		orderGroup.PUT("/:id/cancel", controller.CancelOrderHandler)
		orderGroup.PUT("/:id/status", authMiddleware.RoleBasedAccess("admin"), controller.UpdateOrderStatusHandler)
	}
}
