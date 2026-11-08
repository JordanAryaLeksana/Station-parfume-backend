package config

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
)

var DB *gorm.DB

func ConnectDatabase() {
	dsn := "host=localhost user=postgres password=Jordanarya123. dbname=station_parfume port=5432 sslmode=disable TimeZone=Asia/Jakarta"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connection established successfully")
	// Schema TIDAK dibuat/diubah saat startup. Terapkan docs/schema/schema.sql
	// (migrasi SQL) ke database sebelum menjalankan aplikasi. AutoMigrate tidak
	// dipakai karena tidak bisa membuat partial unique index, CHECK, dan FK komposit.
	DB = db
}
