// Package response: bentuk JSON sukses yang seragam untuk semua endpoint.
//
//	{"message": "...", "data": <isi>}
//
// Error memakai src/middlewares/Error: {"error": "...", "message": "...", "status": 400}.
package response

import "github.com/gin-gonic/gin"

// Body adalah bentuk response sukses (dipakai juga di dokumentasi OpenAPI).
type Body struct {
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// JSON mengirim {"message", "data"} dengan status HTTP code. data boleh nil.
func JSON(c *gin.Context, code int, message string, data any) {
	c.JSON(code, Body{Message: message, Data: data})
}

// OK = JSON dengan status 200.
func OK(c *gin.Context, message string, data any) { JSON(c, 200, message, data) }

// Created = JSON dengan status 201.
func Created(c *gin.Context, message string, data any) { JSON(c, 201, message, data) }
