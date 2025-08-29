package routes

import (
	"backend/src/modules/cart/controller"
	"fmt"

	"github.com/gin-gonic/gin"
)

func RegisterCartRoutes(rg *gin.RouterGroup) {
	cartGroup := rg.Group("/cart")
	{
		cartGroup.POST("/", controller.CreateCartHandler)
		cartGroup.GET("/:id", controller.GetCartByIDHandler)
		cartGroup.POST("/:id/items", controller.AddItemToCartHandler)
		cartGroup.DELETE("/:id", controller.ClearCartHandler)
	}
	fmt.Println("Cart routes registered")
}