package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/payment/controller"
	"os"

	"github.com/gin-gonic/gin"
)

// RegisterPaymentRoutes: pembayaran Xendit.
//   - POST /orders/:id/pay   (JWT, pemilik order) → checkout_url Xendit
//   - POST /webhooks/xendit  (TANPA JWT; diverifikasi header x-callback-token,
//     status selalu dicek ulang ke Xendit)
func RegisterPaymentRoutes(r *gin.RouterGroup) {
	r.POST("/webhooks/xendit", controller.XenditWebhookHandler)
	r.POST("/orders/:id/pay", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")), controller.PayOrderHandler)
}
