package main

import (
	"backend/src/config"

	"os"

	"github.com/gin-gonic/gin"

	"backend/src/routes"
	"backend/src/utils/AuthHandler"
)

func main() {
	config.InitRedis()
	config.ConnectDatabase()
	config.InitMidtrans()
	// Seeder (src/config/seeders) dijalankan terpisah: go run ./seed — lihat seed/README.md.
	router := gin.Default()
	routes.InitRoutes(router)
	authhandler.InitGoogleOauthConfig()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}
