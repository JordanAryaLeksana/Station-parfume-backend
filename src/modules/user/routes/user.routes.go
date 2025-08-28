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
		userGroup.POST("/", controller.CreateUser, authmiddlewares.RoleBasedAccess("admin"))
		userGroup.GET("/", controller.GetAllUsers, authmiddlewares.RoleBasedAccess("admin"))
		userGroup.GET("/:id", controller.GetUserById, authmiddlewares.RoleBasedAccess("admin"))
		userGroup.PUT("/:id", controller.UpdateUser, authmiddlewares.RoleBasedAccess("admin"))
		userGroup.DELETE("/:id", controller.DeleteUser, authmiddlewares.RoleBasedAccess("admin"))
	}
	fmt.Println("User routes registered")
}