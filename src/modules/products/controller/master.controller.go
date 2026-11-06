package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/products/models"
	"backend/src/modules/products/services"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------- aroma

func GetAllAromas(c *gin.Context) {
	aromas, err := services.GetAllAromas()
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Aromas Retrieved", "aromas": aromas})
}

func GetAromaByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	aroma, err := services.GetAromaByID(id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Aroma Retrieved", "aroma": aroma})
}

func UpdateAroma(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input models.AromaUpdateDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	aroma, err := services.UpdateAroma(id, &input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Aroma Updated", "aroma": aroma})
}

// ---------------------------------------------------------------- brand

func GetAllBrands(c *gin.Context) {
	brands, err := services.GetAllBrands()
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Brands Retrieved", "brands": brands})
}

func CreateBrand(c *gin.Context) {
	var input models.BrandRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	brand, err := services.CreateBrand(&input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(201, gin.H{"message": "Brand Created", "brand": brand})
}

// ---------------------------------------------------------------- bottle

func GetAllBottles(c *gin.Context) {
	bottles, err := services.GetAllBottles()
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Bottles Retrieved", "bottles": bottles})
}

func GetBottleByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	bottle, err := services.GetBottleByID(id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Bottle Retrieved", "bottle": bottle})
}

func CreateBottle(c *gin.Context) {
	var input models.BottleRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	bottle, err := services.CreateBottle(&input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(201, gin.H{"message": "Bottle Created", "bottle": bottle})
}

func UpdateBottle(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input models.BottleRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	bottle, err := services.UpdateBottle(id, &input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Bottle Updated", "bottle": bottle})
}
