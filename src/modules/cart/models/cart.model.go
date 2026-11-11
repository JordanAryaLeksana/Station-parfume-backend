package models

type AddCartItemDTO struct {
	VariantID uint `json:"variant_id" validate:"required"`
	Quantity  int  `json:"quantity" validate:"required,gte=1,lte=999"`
}

type UpdateCartItemDTO struct {
	Quantity int `json:"quantity" validate:"required,gte=1,lte=999"`
}

// CartItemResponseDTO: harga selalu dihitung live dari katalog.
type CartItemResponseDTO struct {
	ID             uint    `json:"id"`
	VariantID      uint    `json:"variant_id"`
	ProductID      uint    `json:"product_id"`
	ProductName    string  `json:"product_name"`
	ProductSlug    string  `json:"product_slug"`
	Grade          *string `json:"grade,omitempty"`
	VariantName    string  `json:"variant_name"` // "Dolly Girl, 30 ml, Murni"
	VariantCode    string  `json:"variant_code"`
	ImageURL       *string `json:"image_url,omitempty"`
	UnitPrice      int64   `json:"unit_price"`                 // harga per unit
	CompareAtPrice *int64  `json:"compare_at_price,omitempty"` // harga coret
	Quantity       int     `json:"quantity"`
	Subtotal       int64   `json:"subtotal"`
	Stock          *int    `json:"stock"`     // stok tersedia; null = tidak dihitung (selalu tersedia)
	Available      bool    `json:"available"` // false = varian nonaktif / produk tidak dijual / stok kurang
}

type CartResponseDTO struct {
	ID            uint                  `json:"id"`
	Items         []CartItemResponseDTO `json:"items"`
	TotalQuantity int                   `json:"total_quantity"` // hanya item yang available
	TotalPrice    int64                 `json:"total_price"`    // hanya item yang available
}
