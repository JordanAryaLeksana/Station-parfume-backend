package main

import (
    "os"
    "github.com/gin-gonic/gin"
    "backend/src/config"
    // "backend/src/routes"
    "backend/src/utils/AuthHandler"
    "backend/src/routes"
)

func main() {
    config.ConnectDatabase()
    router := gin.Default()
    routes.InitRoutes(router)
    authhandler.InitGoogleOauthConfig()
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    router.Run(":" + port)
}
