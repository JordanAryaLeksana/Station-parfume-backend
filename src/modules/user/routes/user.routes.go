package routes

import (
	"backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/user/controller"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(router *gin.RouterGroup){
	userGroup := router.Group("/users", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")))
	fmt.Println("Registering user routes...")
	{
		userGroup.POST("/", controller.CreateUser)
		userGroup.GET("/", controller.GetAllUsers)
		userGroup.GET("/:id", controller.GetUserById)
		userGroup.PUT("/:id", controller.UpdateUser)
		userGroup.DELETE("/:id", controller.DeleteUser)
	}
	fmt.Println("User routes registered")
}