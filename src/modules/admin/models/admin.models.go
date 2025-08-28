package models


type AdminRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type AdminResponse struct {
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Role     string  `json:"role"`
}