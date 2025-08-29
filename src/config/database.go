package config

import (
	"backend/src/repository"
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
	db.AutoMigrate(
		&repository.User{},
		&repository.Parfume{},
		&repository.Brand{},
		&repository.Payment{},
		&repository.Cart{},
		&repository.Order{},
		&repository.Bottle{},
		&repository.CartItem{},
		&repository.OrderItem{},
		&repository.TypeBottle{},
	)
	db.Migrator().DropColumn(&repository.Brand{}, "parfume_id")
	db.Migrator().DropColumn(&repository.Parfume{}, "Type")
	db.Migrator().DropColumn(&repository.Parfume{}, "Category")

	DB = db
}
