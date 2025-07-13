package routes

import (
	"backend/src/modules/user/controller"
	"fmt"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(router *gin.RouterGroup){
	userGroup := router.Group("/users")
	{
		userGroup.POST("/", controller.CreateUser)
		userGroup.GET("/", controller.GetAllUsers)
		userGroup.GET("/:id", controller.GetUserById)
		userGroup.PUT("/:id", controller.UpdateUser)
		userGroup.DELETE("/:id", controller.DeleteUser)
	}
	fmt.Println("User routes registered")
}