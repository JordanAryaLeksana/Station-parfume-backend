package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/Order/models"
	"backend/src/modules/Order/services"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CreateOrderHandler(c *gin.Context) {
    var input models.OrderRequestDTO
    if err := c.ShouldBindJSON(&input); err != nil {
        httperror.BadRequestError(c, "Invalid request body: " + err.Error())
        return
    }

    if input.UserID == 0 {
        httperror.BadRequestError(c, "UserID is required")
        return
    }
    if len(input.Items) == 0 {
        httperror.BadRequestError(c, "Order items cannot be empty")
        return
    }

    result, err  := services.CreateOrder(input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
    c.JSON(201, gin.H{
        "message": "Order created successfully",
        "data":    result,
    })
}
func GetOrderByIDHandler(c *gin.Context) {
	idParam := c.Param("id")
	orderID, err := strconv.Atoi(idParam)
	if err != nil {
		httperror.BadRequestError(c, "invalid order id")
		return
	}

	result, err := services.GetOrderByID(uint(orderID))
	if err != nil {
		httperror.NotFoundError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{"data": result})
}

func GetOrdersByUserHandler(c *gin.Context) {
	userIDParam := c.Param("user_id")
	userID, err := strconv.Atoi(userIDParam)
	if err != nil {
		httperror.BadRequestError(c, "invalid user id")
		return
	}

	result, err := services.GetOrdersByUserID(uint(userID))
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{"message": "success","data": result})
}


func CancelOrderHandler(c *gin.Context) {
	idParam := c.Param("id")
	orderID, err := strconv.Atoi(idParam)
	if err != nil {
		httperror.BadRequestError(c, "invalid order id")
		return
	}

	userIDParam := c.Param("user_id")
	userID, err := strconv.Atoi(userIDParam)
	if err != nil {
		httperror.BadRequestError(c, "invalid user id")
		return
	}

	err = services.CancelOrder(uint(orderID), uint(userID))
	if err != nil {
		httperror.BadRequestError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{"message": "order canceled successfully"})
}

func UpdateOrderStatusHandler(c *gin.Context) {
	idParam := c.Param("id")
	orderID, err := strconv.Atoi(idParam)
	if err != nil {
		httperror.BadRequestError(c, "invalid order id")
		return
	}

	var input models.OrderStatusDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, err.Error())
		return
	}

	err = services.UpdateOrderStatus(uint(orderID), input.Name)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{"message": "order status updated successfully"})
}