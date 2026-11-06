package main

import (
	"backend/src/config"

	"os"

	"github.com/gin-gonic/gin"

	// "backend/src/routes"
	"backend/src/routes"
	"backend/src/utils/AuthHandler"
)

func main() {
	config.InitRedis()
	config.ConnectDatabase()
	// Seeder (src/config/seeders) dijalankan terpisah: go run ./cmd/seed — lihat seed/README.md.
	router := gin.Default()
	routes.InitRoutes(router)
	authhandler.InitGoogleOauthConfig()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}
