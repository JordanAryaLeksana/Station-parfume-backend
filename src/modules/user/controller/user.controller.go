package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/user/services"
	"backend/src/modules/user/models"
	"github.com/gin-gonic/gin"
)

func CreateUser(c *gin.Context) {
	var input models.UserDTORequest
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}

	userResponse, err := services.CreateUser(&input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}

	c.JSON(201, gin.H{
		"message": "User Created",
		"user":    userResponse,
	})
}

func GetAllUsers(c *gin.Context) {
	users, err := services.GetAllUsers()
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"users": users,
	})
}

func GetUserById(c *gin.Context) {
	id := c.Param("id")
	user, err := services.GetUserById(id)
	if err != nil {
		httperror.NotFoundError(c, "User not found")
		return
	}
	c.JSON(200, gin.H{
		"user": user,
	})
}

func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var input models.UserDTORequest
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}

	userResponse, err := services.UpdateUser(id, &input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}

	c.JSON(200, gin.H{
		"message": "User Updated",
		"user":    userResponse,
	})
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeleteUser(id); err != nil {
		httperror.NotFoundError(c, "User not found")
		return
	}
	c.JSON(200, gin.H{
		"message": "User Deleted",
	})
}
