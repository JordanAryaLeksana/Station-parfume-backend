package models

import (
	"time"
)

var BottleType = []string{"spray", "roll-on"}
var BottleSize = []string{}

type BottleRequestDTO struct {
	Name        string  `json:"name" validate:"required"`
	Description string  `json:"description" validate:"required"`	
	Size        float64 `json:"bottlesize" validate:"required,gt=0"`
	BottleType  string  `json:"bottle_type" validate:"required,oneof=spray roll-on"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	Image       string  `json:"image" validate:"required"`
	IsNew       bool    `json:"is_new"`
	IsFavorite  bool    `json:"is_favorite"`
	TypeBottleID uint    `json:"type_bottle_id" validate:"required,gt=0"`

}

type BottleResponseDTO struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Size        float64   `json:"bottle_size"`
	Price       float64   `json:"price"`
	Image       string    `json:"image"`
	IsNew       bool      `json:"is_new"`
	IsFavorite  bool      `json:"is_favorite"`
	TypeBottle  TypeBottle `json:"type_bottle"`
	TypeBottleID uint      `json:"type_bottle_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TypeBottle struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"unique;not null" json:"name"`
}