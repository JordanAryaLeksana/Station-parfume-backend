package routes

import (
	"backend/src/modules/admin/controller"
	"fmt"
	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(router *gin.RouterGroup){
	AdminGroup := router.Group("/admins")
	{
		AdminGroup.POST("/", controller.CreateAdminHandler)
		AdminGroup.GET("/:id", controller.GetAdminByIDHandler)
		AdminGroup.PATCH("/:id", controller.UpdateAdminHandler)
		AdminGroup.DELETE("/:id", controller.DeleteAdminHandler)
	}
	fmt.Println("Admin routes registered")
}