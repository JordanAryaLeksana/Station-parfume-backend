package routes

import (
	"backend/src/modules/Order/controller"
	"os"
	authMiddleware "backend/src/middlewares/AuthMiddlewares"
	"github.com/gin-gonic/gin"
)

func RegisterOrderRoutes(rg *gin.RouterGroup) {
	orderGroup := rg.Group("/orders", authMiddleware.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		orderGroup.POST("/", controller.CreateOrderHandler)
		orderGroup.GET("/:id", controller.GetOrderByIDHandler)
		orderGroup.GET("/user/:user_id", controller.GetOrdersByUserHandler)
		orderGroup.PUT("/:id/cancel", controller.CancelOrderHandler)
		orderGroup.PUT("/:id/status", controller.UpdateOrderStatusHandler, authMiddleware.RoleBasedAccess("admin"))
	}
}