package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/order/models"
	"backend/src/modules/order/services"
	paymentservices "backend/src/modules/payment/services"
	"backend/src/repository"
	"backend/src/utils/response"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Semua handler user memakai userID dari token, bukan dari body/URL.

func CheckoutHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	var input models.CheckoutRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid request body")
		return
	}
	order, err := services.Checkout(userID, &input)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, "Order created successfully", order)
}

func GetMyOrdersHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	orders, err := services.GetOrdersByUser(userID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Orders retrieved successfully", orders)
}

func GetOrderByIDHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	// :id boleh ID angka atau nomor pesanan (SP…)
	order, err := services.FindOrderForUser(c.Param("id"), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Order retrieved successfully", order)
}

func CancelOrderHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	orderID, ok := parseID(c)
	if !ok {
		return
	}
	var input models.CancelOrderDTO
	_ = c.ShouldBindJSON(&input) // body opsional
	if err := services.CancelOrder(orderID, userID, input.Reason); err != nil {
		writeError(c, err)
		return
	}
	// Transaksi Midtrans yang masih pending ikut di-expire (best effort).
	paymentservices.CancelOrderSessions(c.Request.Context(), orderID)
	response.OK(c, "Order canceled successfully", nil)
}

func UpdateOrderStatusHandler(c *gin.Context) {
	orderID, ok := parseID(c)
	if !ok {
		return
	}
	var input models.UpdateOrderStatusDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid request body")
		return
	}
	to, err := services.UpdateOrderStatus(orderID, c.GetString("role"), &input)
	if err != nil {
		writeError(c, err)
		return
	}
	if to == repository.OrderCancelled {
		// Sama seperti pembatalan oleh pembeli: transaksi Midtrans yang masih pending di-expire.
		paymentservices.CancelOrderSessions(c.Request.Context(), orderID)
	}
	response.OK(c, "Order status updated successfully", nil)
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

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httperror.BadRequestError(c, "invalid order id")
		return 0, false
	}
	return uint(id), true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		httperror.NotFoundError(c, err.Error())
	case errors.Is(err, services.ErrValidation):
		httperror.BadRequestError(c, err.Error())
	case errors.Is(err, services.ErrForbidden):
		httperror.ForbiddenError(c, err.Error())
	default:
		httperror.InternalServerError(c, "Internal server error")
	}
}
