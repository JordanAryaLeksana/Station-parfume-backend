package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/Payment/controller"
	"fmt"
	"os"
	"github.com/gin-gonic/gin"
)

func RegisterPaymentRoutes(r *gin.RouterGroup) {
	PaymentGroup := r.Group("/payments", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		PaymentGroup.POST("/", controller.CreatePaymentHandler)
		PaymentGroup.POST("/notification", controller.PaymentNotificationHandler)
		PaymentGroup.GET("/:id", controller.GetPaymentHandler)
	}
	fmt.Println("Payment routes registered")
}