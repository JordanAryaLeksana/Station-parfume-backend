package models

type UserDTORequest struct {
	Name    string `gorm:"not null" json:"name" validate:"required"`
	Email   string `gorm:"not null;uniqueIndex" json:"email" validate:"required,email"`
	Password string `gorm:"not null" json:"password" validate:"required,min=8"`
	Role    string `gorm:"not null;default:'user'" json:"role"`
	Picture string `gorm:"" json:"picture"`
}


type UserDTOResponse struct {
	ID    uint   `json:"id"`
	Name  string `gorm:"not null" json:"name"`
	Email string `gorm:"not null;uniqueIndex" json:"email"`
	Role  string `gorm:"not null;default:'user'" json:"role"`
	Picture  string `gorm:"" json:"picture"`
	Provider string `gorm:"" json:"provider"`
	Sub   *string `gorm:"not null;uniqueIndex" json:"sub"`
}


