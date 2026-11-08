package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/admin/controller"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

// Semua endpoint /admins hanya untuk admin yang sudah login.
// Admin pertama dibuat lewat seeder: go run ./seed -admin
func RegisterAdminRoutes(router *gin.RouterGroup) {
	AdminGroup := router.Group("/admins",
		authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")),
		authmiddlewares.RoleBasedAccess("admin"),
	)
	{
		AdminGroup.POST("/", controller.CreateAdminHandler)
		AdminGroup.GET("/:id", controller.GetAdminByIDHandler)
		AdminGroup.PATCH("/:id", controller.UpdateAdminHandler)
		AdminGroup.DELETE("/:id", controller.DeleteAdminHandler)
	}
	fmt.Println("Admin routes registered")
}
