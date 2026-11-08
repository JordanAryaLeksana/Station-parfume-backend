package controller

import (
	"backend/src/modules/products/services"

	"github.com/gin-gonic/gin"
)

func GetAllCategories(c *gin.Context) {
	categories, err := services.GetAllCategories()
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Categories Retrieved", "categories": categories})
}
