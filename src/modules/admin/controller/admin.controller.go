package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/admin/models"
	"backend/src/modules/admin/services"

	"github.com/gin-gonic/gin"
)

func CreateAdminHandler(c *gin.Context){
	var input = models.AdminRequest{}
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid request payload")
		return
	}
	admin, err := services.CreateAdmin(&input)
	if err != nil {
		httperror.InternalServerError(c, "Failed to create admin")
		return
	}
	c.JSON(201, gin.H{
		"message": "Admin Created",
		"admin":   admin,
	})
}

func UpdateAdminHandler(c *gin.Context){
	id := c.Param("id")
	var input models.AdminRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}

	adminResponse, err := services.UpdateAdmin(id, &input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}

	c.JSON(200, gin.H{
		"message": "Admin Updated",
		"admin":   adminResponse,
	})
}

func GetAdminByIDHandler(c *gin.Context){
	paramID := c.Param("id")
	admin, err := services.GetAdminByID(paramID)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Admin Retrieved",
		"admin":   admin,
	})
}

func DeleteAdminHandler(c *gin.Context){
	paramID := c.Param("id")
	if err := services.DeleteAdmin(paramID); err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Admin Deleted",
	})
}
