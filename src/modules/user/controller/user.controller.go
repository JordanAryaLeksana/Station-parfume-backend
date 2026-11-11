package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/user/models"
	"backend/src/modules/user/services"
	"backend/src/utils/response"
	"errors"
	"log"

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
	response.Created(c, "User Created", userResponse)
}

func GetAllUsers(c *gin.Context) {
	users, err := services.GetAllUsers()
	if err != nil {
		log.Printf("users: %v", err)
		httperror.InternalServerError(c, "Internal server error")
		return
	}
	response.OK(c, "Users Retrieved", users)
}

func GetUserById(c *gin.Context) {
	id := c.Param("id")
	user, err := services.GetUserById(id)
	if err != nil {
		httperror.NotFoundError(c, "User not found")
		return
	}
	response.OK(c, "User Retrieved", user)
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

	response.OK(c, "User Updated", userResponse)
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeleteUser(c.GetString("role"), id); err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "User Deleted", nil)
}
