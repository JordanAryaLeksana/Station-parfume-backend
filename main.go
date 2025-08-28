package main

import (
	"backend/src/config"
	// "backend/src/config/seeders"

	"os"

	"github.com/gin-gonic/gin"

	// "backend/src/routes"
	"backend/src/routes"
	"backend/src/utils/AuthHandler"
)

func main() {
    config.InitRedis()
    config.ConnectDatabase()
    // seeders.SeedBottles(config.DB)
    // seeders.SeedBrand(config.DB)
    // seeders.SeedParfume(config.DB)
    router := gin.Default()
    routes.InitRoutes(router)
    authhandler.InitGoogleOauthConfig()
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    router.Run(":" + port)
}
