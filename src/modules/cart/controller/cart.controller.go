package controller

import (
	"backend/src/config"
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/cart/models"
	"backend/src/modules/cart/services"
	"fmt"
	"github.com/gin-gonic/gin"
)

func CreateCartHandler(c *gin.Context)  {
	userID := c.GetUint("userID")
	var input models.CartRequestDTO
	if userID != input.UserID{
		httperror.ForbiddenError(c, "You are not allowed to create a cart for this user")
		return
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input: "+err.Error())
		return 
	}
	cart, err := services.CreateCart(userID, &input)
	if err != nil {
		httperror.InternalServerError(c, "Failed to create cart: "+err.Error())
		return 
	}
	
	c.JSON(201, gin.H{
		"message": "Cart created successfully",
		"data":    cart,
	}) 
}

func GetCartByUserIDHandler(c *gin.Context) {
	userID := c.Param("userID")
	var cart models.CartRequestDTO
	if err := config.DB.Where("id = ?", userID).First(&cart).Error; err != nil {
		httperror.NotFoundError(c, "Cart not found")
		return
	}

	// Convert userID from string to uint
	var userIDUint uint
	if _, err := fmt.Sscanf(userID, "%d", &userIDUint); err != nil {
		httperror.BadRequestError(c, "Invalid user ID format")
		return
	}

	cartResponse, err := services.GetCartByUserID(userIDUint)
	if err != nil {
		httperror.InternalServerError(c, "Failed to retrieve cart: "+err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Cart retrieved successfully",
		"data":    cartResponse,
	})
}


func AddItemToCartHandler(c *gin.Context){
	var input models.CartRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input: "+err.Error())
		return
	}
	var inputItem models.CartItemDTO
	if len(input.Items) == 0 {
		httperror.BadRequestError(c, "No items provided to add")
		return
	}

	if err := c.ShouldBindJSON(&inputItem); err != nil {
		httperror.BadRequestError(c, "Invalid item data: "+err.Error())
		return
	}

	cartID := c.Param("id")
	var cartIDUint uint
	if _, err := fmt.Sscanf(cartID, "%d", &cartIDUint); err != nil {
		httperror.BadRequestError(c, "Invalid cart ID format")
		return
	}
	addtoCartResponse, err := services.AddItemToCart(cartIDUint, inputItem.ParfumeID, inputItem.BottleID, inputItem.Quantity)
	if err != nil {
		httperror.InternalServerError(c, "Failed to add item to cart: "+err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Item added to cart successfully",
		"data":    addtoCartResponse,
	})
}

func ClearCartHandler(c *gin.Context) {
	cartID := c.Param("id")	
	var cartIDUint uint
	if _, err := fmt.Sscanf(cartID, "%d", &cartIDUint); err != nil {
		httperror.BadRequestError(c, "Invalid cart ID format")
		return
	}
	if err := services.ClearCart(cartIDUint); err != nil {
		httperror.InternalServerError(c, "Failed to clear cart: "+err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Cart cleared successfully",
	})
}