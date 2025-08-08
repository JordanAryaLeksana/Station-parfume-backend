package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	"backend/src/modules/products/controller"
	"os"

	"github.com/gin-gonic/gin"
)

func RegisterProductsRoutes(router *gin.RouterGroup) {
	parfumeGroup := router.Group("/parfumes", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		parfumeGroup.POST("/", controller.CreateParfume)
		parfumeGroup.GET("/", controller.GetAllParfumes)
		parfumeGroup.GET("/:id", controller.GetParfumeById)
		parfumeGroup.PUT("/:id", controller.UpdateParfume)
		parfumeGroup.GET("/category/:category", controller.GetParfumesByCategory)
		parfumeGroup.GET("/type/:type", controller.GetParfumeByType)
		parfumeGroup.GET("/brand/:brand", controller.GetParfumeByBrand)
		parfumeGroup.POST("/brand/:brand_id", controller.AddParfumeToBrand)
	}
	brandGroup := router.Group("/brands", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		brandGroup.GET("/", controller.GetAllBrandsHandler)
		brandGroup.POST("/", controller.CreateBrandHandler)
	}
	bottleGroup := router.Group("/bottles", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")))
	{
		bottleGroup.POST("/", controller.CreateBottleHandler)
		bottleGroup.GET("/", controller.GetAllBottlesHandler)
		bottleGroup.GET("/:id", controller.GetBottleByIDHandler)
		bottleGroup.PUT("/:id", controller.UpdateBottleHandler)
	}
}