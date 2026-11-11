package routes

import (
	authMiddleware "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/order/controller"
	"backend/src/repository"
	"github.com/gin-gonic/gin"
	"os"
)

// RegisterOrderRoutes: order milik user di token; /orders/:id/status khusus admin.
// Aman didaftarkan karena middleware auth/role sekarang memanggil Abort.
func RegisterOrderRoutes(rg *gin.RouterGroup) {
	orderGroup := rg.Group("/orders", authMiddleware.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		orderGroup.POST("", controller.CheckoutHandler) // checkout dari keranjang
		orderGroup.GET("", controller.GetMyOrdersHandler)
		orderGroup.GET("/:id", controller.GetOrderByIDHandler)
		orderGroup.PUT("/:id/cancel", controller.CancelOrderHandler)
		orderGroup.PUT("/:id/status", authMiddleware.RoleBasedAccess(repository.RoleOwner, repository.RoleAdmin, repository.RoleWarehouse), controller.UpdateOrderStatusHandler)
	}
}
