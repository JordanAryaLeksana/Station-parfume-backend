package models

import "time"


var ParfumeType = []string{"exclusive", "regular"}

var Category = []string{"mens", "womens", "unisex", "shalat"}
type ParfumeRequestDTO struct {
	Name        string  `json:"name" validate:"required"`
	BrandID     uint    `json:"brand_id" validate:"required, gt=0"`
	Description string  `json:"description" validate:"required"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	Image       string  `json:"image" validate:"required"`
	TypeID      uint    `json:"type_id" validate:"required"`
	CategoryID  uint    `json:"category_id" validate:"required"`
	Favorite    bool    `json:"favorite"`
}

type ParfumeResponseDTO struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	BrandID     uint      `json:"brand_id"`
	Brand       BrandDTO  `json:"brand"` 
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Image       string    `json:"image"`
	Type        TypeDTO   `json:"type"`
	TypeID      uint      `json:"type_id"`
	Category    CategoriesDTO `json:"category"`
	CategoryID  uint      `json:"category_id"`
	Favorite    bool      `json:"favorite"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CategoriesDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type TypeDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type BrandDTO struct {
	ID          uint   `json:"id"`
	Name        string `gorm:"not null;size:255" json:"name"`
	Description string `gorm:"not null" json:"description"`
	Logo        string `gorm:"not null" json:"logo"`
}
