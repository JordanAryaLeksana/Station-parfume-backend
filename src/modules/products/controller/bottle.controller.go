package controller
import (
	"backend/src/modules/products/models"
	"backend/src/modules/products/services"
	httperror "backend/src/middlewares/Error"
	"github.com/gin-gonic/gin"
	"strconv"
)


func CreateBottleHandler(c *gin.Context) {
	var input models.BottleRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	bottleResponse, err := services.CreateBottle(&input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(201, gin.H{
		"message": "Bottle Created",
		"bottle":  bottleResponse,
	})
}

func GetAllBottlesHandler(c *gin.Context) {
	bottles, err := services.GetAllBottles();
	if err != nil {
		httperror.BadRequestError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Bottles Retrieved",
		"bottles": bottles,
	})
}	

func GetBottleByIDHandler(c *gin.Context) {
	idStr := c.Param("id")
	idUint64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httperror.BadRequestError(c, "Invalid bottle ID")
		return
	}
	id := uint(idUint64)
	bottle, err := services.GetBottleByID(id)
	if err != nil {
		httperror.NotFoundError(c, "Bottle not found")
		return
	}
	c.JSON(200, gin.H{
		"message": "Bottle Retrieved",
		"bottle":  bottle,
	})
}

func UpdateBottleHandler(c *gin.Context) {
	idStr := c.Param("id")
	idUint64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httperror.BadRequestError(c, "Invalid bottle ID")
		return
	}
	id := uint(idUint64)
	var input models.BottleRequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		httperror.BadRequestError(c, "Invalid input data")
		return
	}
	bottleResponse, err := services.UpdateBottle(id, &input)
	if err != nil {
		httperror.InternalServerError(c, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"message": "Bottle Updated",
		"bottle":  bottleResponse,
	})
}
