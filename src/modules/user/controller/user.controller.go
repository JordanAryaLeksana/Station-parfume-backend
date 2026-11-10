package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/user/models"
	"backend/src/modules/user/services"
	"errors"

	"github.com/gin-gonic/gin"
)

// writeError memetakan error service ke status HTTP.
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrForbidden):
		httperror.ForbiddenError(c, err.Error())
	case errors.Is(err, services.ErrNotFound):
		httperror.NotFoundError(c, "User not found")
	case errors.Is(err, services.ErrValidation):
		httperror.BadRequestError(c, err.Error())
	default:
		httperror.InternalServerError(c, "Internal server error")
	}
}

func CreateUser(c *gin.Context) {
	var input models.UserDTORequest
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}

	userResponse, err := services.CreateUser(c.GetString("role"), &input)
	if err != nil {
		writeError(c, err)
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

	userResponse, err := services.UpdateUser(c.GetString("role"), id, &input)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"message": "User Updated",
		"user":    userResponse,
	})
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeleteUser(c.GetString("role"), id); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{
		"message": "User Deleted",
	})
}
