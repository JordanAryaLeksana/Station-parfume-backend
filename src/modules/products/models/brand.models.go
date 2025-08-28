package models

import (
	"time"
)

type BrandRequestDTO struct {
	Name        string `gorm:"not null;size:255" json:"name" validate:"required"`
	Description string `gorm:"not null" json:"description" validate:"required"`
	Logo        string `gorm:"not null" json:"logo" validate:"required"`
}

type BrandResponseDTO struct{ 
	ID          uint      `json:"id"`
	Name        string    `gorm:"not null;size:255" json:"name"`
	Description string    `gorm:"not null" json:"description"`
	Logo        string    `gorm:"not null" json:"logo"`
	Parfume     []Parfume `gorm:"foreignKey:BrandID" json:"parfume"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type Parfume struct {
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



