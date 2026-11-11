package routes

import (
	"backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/user/controller"
	"backend/src/repository"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

// Manajemen user hanya untuk admin. Cek role dipasang di level grup,
// jadi selalu jalan SEBELUM handler.
func RegisterUserRoutes(router *gin.RouterGroup) {
	userGroup := router.Group("/users",
		authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")),
		authmiddlewares.RoleBasedAccess(repository.RoleOwner, repository.RoleAdmin),
	)
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
