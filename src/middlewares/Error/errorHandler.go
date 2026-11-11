package httperror

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func BadRequestError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
		"error":   "Bad Request",
		"message": message,
		"status":  http.StatusBadRequest,
	})
}

func InternalServerError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
		"error":   "Internal Server Error",
		"message": message,
		"status":  http.StatusInternalServerError,
	})
}

func NotFoundError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
		"error":   "Not Found",
		"message": message,
		"status":  http.StatusNotFound,
	})
}

func UnauthorizedError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error":   "Unauthorized",
		"message": message,
		"status":  http.StatusUnauthorized,
	})
}

func ForbiddenError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error":   "Forbidden",
		"message": message,
		"status":  http.StatusForbidden,
	})
}
func ConflictError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusConflict, gin.H{
		"error":   "Conflict",
		"message": message,
		"status":  http.StatusConflict,
	})
}

func UnprocessableEntityError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
		"error":   "Unprocessable Entity",
		"message": message,
		"status":  http.StatusUnprocessableEntity,
	})
}

func ServiceUnavailableError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
		"error":   "Service Unavailable",
		"message": message,
		"status":  http.StatusServiceUnavailable,
	})
}

func TooManyRequestsError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error":   "Too Many Requests",
		"message": message,
		"status":  http.StatusTooManyRequests,
	})
}

func NotImplementedError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
		"error":   "Not Implemented",
		"message": message,
		"status":  http.StatusNotImplemented,
	})
}

func GatewayTimeoutError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{
		"error":   "Gateway Timeout",
		"message": message,
		"status":  http.StatusGatewayTimeout,
	})
}

func MethodNotAllowedError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
		"error":   "Method Not Allowed",
		"message": message,
		"status":  http.StatusMethodNotAllowed,
	})
}

func NotAcceptableError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{
		"error":   "Not Acceptable",
		"message": message,
		"status":  http.StatusNotAcceptable,
	})
}

func PayloadTooLargeError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
		"error":   "Request Entity Too Large",
		"message": message,
		"status":  http.StatusRequestEntityTooLarge,
	})
}

func BadGatewayError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
		"error":   "Bad Gateway",
		"message": message,
		"status":  http.StatusBadGateway,
	})
}
