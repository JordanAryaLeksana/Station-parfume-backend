package controller

import (
	"backend/src/modules/parfume/models"
	"backend/src/modules/parfume/services"
	httperror"backend/src/middlewares/Error"
	"github.com/gin-gonic/gin"
	"strconv"
)

func CreateParfume(c *gin.Context) {
	var input models.ParfumeRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	parfumeResponse, err := services.CreateParfume(input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(201, gin.H{
		"message": "Parfume Created",
		"parfume": parfumeResponse,
	})
}

func GetAllParfumes(c *gin.Context) {
	parfumes, err := services.GetAllParfumes()
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Parfumes Retrieved",
		"parfumes": parfumes,
	})
}

func GetParfumeById(c *gin.Context) {
	id := c.Param("id")
	parfume, err := services.GetParfumeById(id)
	if err != nil {
		httperror.NotFoundError(c, "Parfume not found")
		return
	}
	c.JSON(200, gin.H{
		"message": "Parfume Retrieved",
		"parfume": parfume,
	})
}

func UpdateParfume(c *gin.Context) {
	id := c.Param("id")
	var input models.ParfumeRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	parfumeResponse, err := services.UpdateParfume(id, input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Parfume Updated",
		"parfume": parfumeResponse,
	})
}

func GetParfumesByCategory(c *gin.Context) {
	category := c.Param("category")
	parfumes, err := services.SortParfumesByCategory(category)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Parfumes Retrieved by Category",
		"parfumes": parfumes,
	})
}

func GetParfumeByType(c *gin.Context) {
	typeName := c.Param("type")
	parfumes, err := services.SortParfumeByType(typeName)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Parfumes Retrieved by Type",
		"parfumes": parfumes,
	})
}

func GetParfumeByBrand(c *gin.Context) {
	brandName := c.Param("brand")
	parfumes, err := services.SortParfumesByBrand(brandName)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Parfumes Retrieved by Brand",
		"parfumes": parfumes,
	})
}


func AddParfumeToBrand(c *gin.Context) {
	brandID := c.Param("brand_id")
	var input models.ParfumeRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	brandIDUint32, err := strconv.ParseUint(brandID, 10, 32)
	if err != nil {
		httperror.BadRequestError(c, "Invalid brand ID")
		return
	}
	brandIDUint := uint(brandIDUint32)
	parfumeResponse, err := services.AddParfumeToBrand(brandIDUint, input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(201, gin.H{
		"message": "Parfume Added to Brand",
		"parfume": parfumeResponse,
	})
}