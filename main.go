package main

import (
    "os"
    "github.com/gin-gonic/gin"
    "backend/src/config"
    "backend/src/routes"
    "backend/src/utils/AuthHandler"
)

func main() {
    config.ConnectDatabase()

    router := gin.Default()

    api := router.Group("/api/v1")

	api.GET("/", func(c* gin.Context){
		c.JSON(200, gin.H{"message": "Welcome to Station Parfume API!"})
	})
    // routes.RegisterRoutes(api) 
    authhandler.InitGoogleOauthConfig()

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    router.Run(":" + port)
}
