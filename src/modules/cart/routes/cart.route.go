package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/cart/controller"
	"fmt"
	"os"
	"github.com/gin-gonic/gin"
)

func RegisterCartRoutes(rg *gin.RouterGroup) {
	cartGroup := rg.Group("/cart", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		cartGroup.POST("/", controller.CreateCartHandler)
		cartGroup.GET("/:id", controller.GetCartByUserIDHandler)
		cartGroup.POST("/:id/items", controller.AddItemToCartHandler)
		cartGroup.DELETE("/:id", controller.ClearCartHandler)
	}
	fmt.Println("Cart routes registered")
}