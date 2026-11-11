package controller

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/modules/products/services"
	"backend/src/utils/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// staffID: id staf dari token (AuthMiddleware sudah memastikan ada).
func staffID(c *gin.Context) uint { return c.GetUint("userID") }

func bind[T any](c *gin.Context) (*T, bool) {
	var in T
	if err := c.ShouldBindJSON(&in); err != nil {
		httperror.BadRequestError(c, "Invalid request body: "+err.Error())
		return nil, false
	}
	return &in, true
}

func AdminListProducts(c *gin.Context) {
	out, err := services.AdminListProducts(c.Query("status"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Products Retrieved", out)
}

func AdminGetProduct(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := services.AdminGetProduct(id)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Product Retrieved", out)
}

func AdminCreateProduct(c *gin.Context) {
	in, ok := bind[services.ProductInput](c)
	if !ok {
		return
	}
	out, err := services.AdminCreateProduct(staffID(c), in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, "Product Created", out)
}

func AdminUpdateProduct(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.ProductPatch](c)
	if !ok {
		return
	}
	out, err := services.AdminUpdateProduct(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Product Updated", out)
}

func AdminAddScent(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.ScentInput](c)
	if !ok {
		return
	}
	out, err := services.AdminAddScent(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, "Scent Created", out)
}

func AdminUpdateScent(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.ScentPatch](c)
	if !ok {
		return
	}
	out, err := services.AdminUpdateScent(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Scent Updated", out)
}

func AdminGenerateVariants(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	n, err := services.AdminGenerateVariants(staffID(c), id)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Variants Generated", gin.H{"created": n})
}

func AdminAddGeneralVariant(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.GeneralVariantInput](c)
	if !ok {
		return
	}
	out, err := services.AdminAddGeneralVariant(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, "Variant Created", out)
}

// AdminUpdateVariants: PATCH /admin/variants  body: {"variants":[{"id":1,"price":50000,"is_active":true}, ...]}
func AdminUpdateVariants(c *gin.Context) {
	in, ok := bind[struct {
		Variants []services.VariantPatch `json:"variants"`
	}](c)
	if !ok {
		return
	}
	n, err := services.AdminUpdateVariants(staffID(c), in.Variants)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Variants Updated", gin.H{"updated": n})
}

func AdminListSizes(c *gin.Context) {
	out, err := services.AdminListSizes()
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Sizes Retrieved", out)
}

func AdminCreateSize(c *gin.Context) {
	in, ok := bind[services.SizeInput](c)
	if !ok {
		return
	}
	out, err := services.AdminCreateSize(staffID(c), in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, "Size Created", out)
}

func AdminUpdateSize(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.SizePatch](c)
	if !ok {
		return
	}
	out, err := services.AdminUpdateSize(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Size Updated", out)
}

// AdminAdjustInventory: POST /admin/inventory/adjustments
// body: {"adjustments":[{"variant_id":1,"mode":"add","quantity":10,"note":"restock supplier"}]}
func AdminAdjustInventory(c *gin.Context) {
	in, ok := bind[struct {
		Adjustments []services.InventoryAdjustment `json:"adjustments"`
	}](c)
	if !ok {
		return
	}
	out, err := services.AdminAdjustInventory(staffID(c), in.Adjustments)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Inventory Adjusted", out)
}

// AdminListMovements: GET /admin/inventory/movements?variant_id=&limit=
func AdminListMovements(c *gin.Context) {
	vid, _ := strconv.ParseUint(c.Query("variant_id"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	out, err := services.AdminListMovements(uint(vid), limit)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Movements Retrieved", out)
}

// AdminSetProductRates: PATCH /admin/products/:id/rates  {"price_per_ml_murni":3000,"price_per_ml_campur":1500}
func AdminSetProductRates(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.RatesInput](c)
	if !ok {
		return
	}
	out, err := services.AdminSetProductRates(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Rates Updated", out)
}

// AdminSetProductSizes: PUT /admin/products/:id/sizes  {"size_ids":[1,2]}  ([] = ukuran default)
func AdminSetProductSizes(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.ProductSizesInput](c)
	if !ok {
		return
	}
	out, err := services.AdminSetProductSizes(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Sizes Updated", out)
}

// AdminAddImage: POST /admin/products/:id/images  {"url":"https://cdn.../annasui.jpg","scent_id":null,"sort_order":0}
func AdminAddImage(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.ImageInput](c)
	if !ok {
		return
	}
	out, err := services.AdminAddImage(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Created(c, "Image Added", out)
}

func AdminUpdateImage(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	in, ok := bind[services.ImagePatch](c)
	if !ok {
		return
	}
	out, err := services.AdminUpdateImage(staffID(c), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Image Updated", out)
}

func AdminDeleteImage(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := services.AdminDeleteImage(staffID(c), id)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, "Image Deleted", out)
}
