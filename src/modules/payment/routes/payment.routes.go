package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/payment/controller"
	"os"

	"github.com/gin-gonic/gin"
)

// RegisterPaymentRoutes: pembayaran Midtrans Snap.
//   - POST /orders/:id/pay     (JWT, pemilik order) → checkout_url (halaman Snap)
//   - POST /webhooks/midtrans  (TANPA JWT; diverifikasi signature_key di body,
//     status selalu dicek ulang ke Midtrans). Daftarkan URL ini sebagai
//     Payment Notification URL di Dashboard Midtrans.
func RegisterPaymentRoutes(r *gin.RouterGroup) {
	r.POST("/webhooks/midtrans", controller.MidtransWebhookHandler)
	r.POST("/orders/:id/pay", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")), controller.PayOrderHandler)
}
