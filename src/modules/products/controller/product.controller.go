package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/products/services"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetAllProducts: GET /products?category=&kind=&grade=&note=&q=
func GetAllProducts(c *gin.Context) {
	products, err := services.GetAllProducts(services.ProductFilter{
		Category: c.Query("category"), Kind: c.Query("kind"), Grade: c.Query("grade"), Note: c.Query("note"), Query: c.Query("q"),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Products Retrieved", "products": products})
}

// GetProduct: GET /products/:id  (:id boleh angka atau slug)
func GetProduct(c *gin.Context) {
	product, err := services.GetProduct(c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Product Retrieved", "product": product})
}

func GetSizes(c *gin.Context) {
	sizes, err := services.GetSizes()
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Sizes Retrieved", "sizes": sizes})
}

// parseID membaca :id sebagai angka; kalau tidak valid langsung menulis 400.
func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		httperror.BadRequestError(c, "Invalid ID")
		return 0, false
	}
	return uint(id), true
}

// writeError memetakan error service ke status HTTP.
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		httperror.NotFoundError(c, err.Error())
	case errors.Is(err, services.ErrConflict):
		httperror.ConflictError(c, err.Error())
	case errors.Is(err, services.ErrValidation):
		httperror.BadRequestError(c, err.Error())
	default:
		httperror.InternalServerError(c, "Internal server error")
	}
}
