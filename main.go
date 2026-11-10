package main

import (
	"backend/src/config"
	paymentservices "backend/src/modules/payment/services"
	"context"
	"log"
	"time"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"backend/src/routes"
	"backend/src/utils/AuthHandler"
)

func main() {
	// .env (khusus dev) dimuat PALING AWAL, sebelum config apa pun membaca env.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found (memakai environment variable sistem)")
	}
	config.InitRedis()
	config.ConnectDatabase()
	// Pembayaran: Xendit Payment Sessions. Kalau env belum lengkap, server tetap
	// jalan tapi endpoint bayar & webhook menjawab 503.
	if err := paymentservices.ConfigureFromEnv(); err != nil {
		log.Printf("PERINGATAN pembayaran: %v", err)
	}
	// Seeder (src/config/seeders) dijalankan terpisah: go run ./seed — lihat seed/README.md.
	go reconcileLoop(5 * time.Minute)
	router := gin.Default()
	routes.InitRoutes(router)
	authhandler.InitGoogleOauthConfig()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}

// reconcileLoop berkala: cek pembayaran yang webhook-nya terlewat, lalu order yang
// lewat batas bayar dibatalkan sesi Xendit-nya dan ditandai EXPIRED (reservasi
// stok dilepas).
func reconcileLoop(every time.Duration) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if n, err := paymentservices.ReconcileAndExpire(ctx, time.Now()); err != nil {
			log.Printf("rekonsiliasi pembayaran: %v", err)
		} else if n > 0 {
			log.Printf("rekonsiliasi pembayaran: %d order kedaluwarsa", n)
		}
		cancel()
		time.Sleep(every)
	}
}
