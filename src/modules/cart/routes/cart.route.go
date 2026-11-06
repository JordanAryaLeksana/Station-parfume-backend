package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/cart/controller"
	"fmt"
	"github.com/gin-gonic/gin"
	"os"
)

// Keranjang selalu milik user di token: tidak ada :userID / :cartID di URL.
func RegisterCartRoutes(rg *gin.RouterGroup) {
	cartGroup := rg.Group("/cart", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		cartGroup.GET("/", controller.GetCartHandler)
		cartGroup.DELETE("/", controller.ClearCartHandler)
		cartGroup.POST("/items", controller.AddItemToCartHandler)
		cartGroup.PUT("/items/:id", controller.UpdateCartItemHandler)
		cartGroup.DELETE("/items/:id", controller.RemoveCartItemHandler)
	}
	fmt.Println("Cart routes registered")
}
