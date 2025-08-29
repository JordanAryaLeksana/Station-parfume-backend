package seeders

import (
	"backend/src/repository"
	"fmt"
	"os"
	"gorm.io/gorm"
)

func SeedAdmin(db *gorm.DB){
	user := repository.User{
		Name:     os.Getenv("ADMIN_NAME"),
		Email:    os.Getenv("ADMIN_EMAIL"),
		Password: os.Getenv("ADMIN_PASSWORD"),
		Role:     "admin",
	}
	if err := db.Create(&user).Error; err != nil {
		fmt.Printf("failed to seed admin: %v\n", err)
	}
}