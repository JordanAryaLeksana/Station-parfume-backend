package controller

import (
	"backend/src/modules/products/services"
	"backend/src/repository"

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

// GetScentGroups: GET /scents?grade=regular|exclusive&note=woody
func GetScentGroups(c *gin.Context) {
	groups, err := services.GetScentGroups(c.Query("grade"), c.Query("note"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Scents Retrieved", "scent_groups": groups, "notes": repository.ScentNotes})
}
