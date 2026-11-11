package controller

import (
	"backend/src/modules/products/services"
	"backend/src/utils/response"

	"github.com/gin-gonic/gin"
)

func GetAllCategories(c *gin.Context) {
	categories, err := services.GetAllCategories()
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Categories Retrieved", categories)
}
