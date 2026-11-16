package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/payment/controller"
	"os"

	"github.com/gin-gonic/gin"
)

// RegisterPaymentRoutes: pembayaran DOKU Checkout.
//   - POST /orders/:id/pay  (JWT, pemilik order) → checkout_url (halaman DOKU Checkout)
//   - POST /webhooks/doku   (TANPA JWT; diverifikasi header Signature, status selalu
//     dicek ulang ke DOKU). Daftarkan URL ini sebagai Notification URL di dashboard
//     DOKU (Settings → Payment Settings, plus Expired Notification di Checkout).
func RegisterPaymentRoutes(r *gin.RouterGroup) {
	r.POST("/webhooks/doku", controller.DokuWebhookHandler)
	r.POST("/orders/:id/pay", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")), controller.PayOrderHandler)
}
