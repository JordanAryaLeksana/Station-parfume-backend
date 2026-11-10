package main

import (
	"backend/src/config"
	orderservices "backend/src/modules/order/services"
	"log"
	"time"

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
	go expireOrdersLoop(5 * time.Minute)
	router := gin.Default()
	routes.InitRoutes(router)
	authhandler.InitGoogleOauthConfig()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}

// expireOrdersLoop menandai order yang tidak dibayar melewati batas waktu sebagai
// EXPIRED secara berkala, supaya stok yang di-reserve kembali tersedia.
func expireOrdersLoop(every time.Duration) {
	for {
		if n, err := orderservices.ExpireOverdueOrders(time.Now()); err != nil {
			log.Printf("expire orders: %v", err)
		} else if n > 0 {
			log.Printf("expire orders: %d order kedaluwarsa", n)
		}
		time.Sleep(every)
	}
}
