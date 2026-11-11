package routes

import (
	authmiddlewares "backend/src/middlewares/AuthMiddlewares"
	ratelimit "backend/src/middlewares/RateLimit"
	"backend/src/modules/auth/controller"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(router *gin.RouterGroup) {
	auth := router.Group("/auth")
	{
		auth.POST("/register", ratelimit.Limit("register", 5, time.Minute), controller.Register)
		auth.POST("/login", ratelimit.Limit("login", 10, time.Minute), controller.LoginManual)
		auth.POST("/login/admin", ratelimit.Limit("login-admin", 10, time.Minute), controller.LoginAdmin)
		auth.GET("/google", controller.GoogleLogin)
		auth.GET("/google/callback", controller.GoogleCallback)
		auth.POST("/logout", authmiddlewares.AuthMiddleware(os.Getenv("JWT_SECRET")), controller.Logout)
		auth.POST("/refresh", ratelimit.Limit("refresh", 30, time.Minute),
			authmiddlewares.RefreshTokenMiddleware(os.Getenv("JWT_REFRESH_SECRET")), controller.RefreshToken)
	}
}
