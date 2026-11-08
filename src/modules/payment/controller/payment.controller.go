package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/payment/models"
	"backend/src/modules/payment/services"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CreatePaymentHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	var input models.PaymentRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid request body")
		return
	}
	paymentResp, err := services.CreatePayment(userID, &input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Payment created successfully",
		"data":    paymentResp,
	})
}

func PaymentNotificationHandler(c *gin.Context) {
	var notificationPayload map[string]any
	if err := c.ShouldBindJSON(&notificationPayload); err != nil {
		httperror.BadRequestError(c, "Invalid request body")
		return
	}
	paymentResp, err := services.HandleNotification(notificationPayload)
	if err != nil {
		writeError(c, err)
		return
	}
	// Endpoint publik (dipanggil Midtrans): jangan kirim balik detail pembayaran.
	c.JSON(http.StatusOK, gin.H{
		"message": "Payment notification processed successfully",
		"status":  paymentResp.Status,
	})
}

func GetPaymentHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	paymentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paymentID == 0 {
		httperror.BadRequestError(c, "Invalid payment id format")
		return
	}
	paymentResp, err := services.GetPayment(uint(paymentID), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Payment retrieved successfully",
		"data":    paymentResp,
	})
}

func currentUser(c *gin.Context) (uint, bool) {
	userID := c.GetUint("userID")
	if userID == 0 {
		if !c.Writer.Written() {
			httperror.UnauthorizedError(c, "Unauthorized")
		}
		return 0, false
	}
	return userID, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		httperror.NotFoundError(c, err.Error())
	case errors.Is(err, services.ErrValidation):
		httperror.BadRequestError(c, err.Error())
	default:
		httperror.InternalServerError(c, "Internal server error")
	}
}
