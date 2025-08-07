package controller

import (
	"backend/src/modules/parfume/models"
	"backend/src/modules/parfume/services"
	"github.com/gin-gonic/gin"

	httperror "backend/src/middlewares/Error"
)


func CreateBrandHandler(c *gin.Context){
	var input models.BrandRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
	}
	brandResponse, err := services.CreateBrand(&input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(201, gin.H{
		"message": "Brand Created",
		"brand": brandResponse,
	})
}


func GetAllBrandsHandler( c *gin.Context) {
	brands, err := services.GetAllBrands()
	if err != nil {
		httperror.BadRequestError(c, err.Error())
		return
	}	
	c.JSON(200, gin.H{
		"message": "Brands Retrieved",
		"brands": brands,
	})
}

