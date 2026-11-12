package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/payment/services"
	"backend/src/utils/response"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// PayOrderHandler: POST /orders/:id/pay → {payment: {checkout_url, expires_at, status, ...}}.
// Frontend redirect ke checkout_url (atau buka popup snap.js dengan snap_token); setelah
// kembali ke halaman sukses, polling GET /orders/:id.
func PayOrderHandler(c *gin.Context) {
	userID := c.GetUint("userID")
	if userID == 0 {
		httperror.UnauthorizedError(c, "Unauthorized")
		return
	}
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || orderID == 0 {
		httperror.BadRequestError(c, "Invalid order id")
		return
	}
	payment, err := services.PayOrder(c.Request.Context(), userID, uint(orderID))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Payment ready", payment)
}

// maxWebhookBody: batas body notifikasi Midtrans.
const maxWebhookBody = 64 << 10

// MidtransWebhookHandler: POST /webhooks/midtrans (tanpa JWT; diverifikasi
// signature_key di body). HTTP 200 = diterima; selain itu Midtrans mengirim ulang.
func MidtransWebhookHandler(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBody))
	if err != nil {
		httperror.BadRequestError(c, "Body terlalu besar atau tidak terbaca")
		return
	}
	if err := services.HandleWebhook(c.Request.Context(), body); err != nil {
		if !errors.Is(err, services.ErrUnauthorized) {
			log.Printf("webhook midtrans: %v", err)
		}
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"received": true})
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrUnauthorized):
		httperror.UnauthorizedError(c, "invalid signature")
	case errors.Is(err, services.ErrNotFound):
		httperror.NotFoundError(c, err.Error())
	case errors.Is(err, services.ErrValidation):
		httperror.BadRequestError(c, err.Error())
	case errors.Is(err, services.ErrConflict):
		httperror.ConflictError(c, err.Error())
	case errors.Is(err, services.ErrNotConfigured):
		httperror.ServiceUnavailableError(c, "Pembayaran belum tersedia")
	case errors.Is(err, services.ErrRetryLater):
		httperror.ServiceUnavailableError(c, "Belum bisa diproses, kirim ulang nanti")
	case errors.Is(err, services.ErrGateway):
		// Detail error gateway hanya ke log; klien mendapat pesan umum.
		log.Printf("payment: %v", err)
		httperror.BadGatewayError(c, "Gateway pembayaran sedang bermasalah, coba lagi")
	default:
		httperror.InternalServerError(c, "Internal server error")
	}
}
