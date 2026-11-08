package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/payment/controller"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

func RegisterPaymentRoutes(r *gin.RouterGroup) {
	// Webhook Midtrans: TANPA JWT (Midtrans tidak punya token kita). Aman karena
	// payload tidak dipercaya: status selalu dicek ulang ke Midtrans
	// (CheckTransaction) berdasarkan reference_id milik kita.
	r.POST("/payments/notification", controller.PaymentNotificationHandler)

	PaymentGroup := r.Group("/payments", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		PaymentGroup.POST("/", controller.CreatePaymentHandler)
		PaymentGroup.GET("/:id", controller.GetPaymentHandler)
	}
	fmt.Println("Payment routes registered")
}
