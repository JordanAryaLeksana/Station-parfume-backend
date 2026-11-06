package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/cart/models"
	"backend/src/modules/cart/services"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Semua handler memakai userID dari token (di-set AuthMiddleware), bukan dari body/URL.

func GetCartHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	cart, err := services.GetCart(userID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Cart retrieved successfully", "data": cart})
}

func AddItemToCartHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	var input models.AddCartItemDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input")
		return
	}
	cart, err := services.AddItem(userID, &input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Item added to cart successfully", "data": cart})
}

func UpdateCartItemHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	itemID, ok := parseID(c)
	if !ok {
		return
	}
	var input models.UpdateCartItemDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input")
		return
	}
	cart, err := services.UpdateItem(userID, itemID, &input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Cart item updated successfully", "data": cart})
}

func RemoveCartItemHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	itemID, ok := parseID(c)
	if !ok {
		return
	}
	cart, err := services.RemoveItem(userID, itemID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Cart item removed successfully", "data": cart})
}

func ClearCartHandler(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	if err := services.ClearCart(userID); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Cart cleared successfully"})
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
		httperror.BadRequestError(c, "Invalid ID")
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
	default:
		httperror.InternalServerError(c, "Internal server error")
	}
}
