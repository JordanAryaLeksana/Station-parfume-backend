package routes


import (	
	"backend/src/modules/parfume/controller"
	"github.com/gin-gonic/gin"
)

func ParfumeRoutes(router *gin.Engine) {
	parfumeGroup := router.Group("/parfumes")
	{
		parfumeGroup.POST("/", controller.CreateParfume)
		parfumeGroup.GET("/", controller.GetAllParfumes)
		parfumeGroup.GET("/:id", controller.GetParfumeById)
		parfumeGroup.PUT("/:id", controller.UpdateParfume)
		parfumeGroup.GET("/category/:category", controller.GetParfumesByCategory)
		parfumeGroup.GET("/type/:type", controller.GetParfumeByType)
		parfumeGroup.GET("/brand/:brand", controller.GetParfumeByBrand)
	}
	brandGroup := router.Group("/brands")
	{
		brandGroup.GET("/", controller.GetAllBrands)
		brandGroup.GET("/:id", controller.GetBrandById)
		brandGroup.POST("/", controller.CreateBrand)
		brandGroup.PUT("/:id", controller.UpdateBrand)
	}
}	