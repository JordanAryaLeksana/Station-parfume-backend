package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/Payment/models"
	"backend/src/modules/Payment/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CreatePaymentHandler(c *gin.Context) {
	var input models.PaymentRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, err.Error())
		return
	}

	paymentResp, err := services.CreatePayment(&input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment created successfully",
		"data":    paymentResp,
	})
}

func PaymentNotificationHandler(c *gin.Context) {
	var notificationPayload map[string]interface{}
	if err := c.ShouldBindJSON(&notificationPayload); err != nil {
		httperror.BadRequestError(c, err.Error())
		return
	}

	paymentResp, err := services.HandleNotification(notificationPayload)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment notification processed successfully",
		"data":    paymentResp,
	})
}

func GetPaymentHandler(c *gin.Context) {
	var paymentIDStr string = c.Param("id")
	var paymentID uint
	if paymentIDStr != "" {
		if parsedPaymentID, err := strconv.ParseUint(paymentIDStr, 10, 32); err == nil {
			paymentID = uint(parsedPaymentID)
		} else {
			httperror.BadRequestError(c, "Invalid payment id format")
			return
		}
	}
	var OrderIDStr string = c.Query("order_id")
	var OrderID uint
	if OrderIDStr != "" {
		if parsedOrderID, err := strconv.ParseUint(OrderIDStr, 10, 32); err == nil {
			OrderID = uint(parsedOrderID)
		} else {
			httperror.BadRequestError(c, "Invalid order_id format")
			return
		}
	}
	paymentResp, err := services.GetPayment(paymentID, OrderID)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment retrieved successfully",
		"data":    paymentResp,
	})
}