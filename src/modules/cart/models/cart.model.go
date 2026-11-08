package models

type AddCartItemDTO struct {
	ProductModelID uint `json:"product_model_id" validate:"required"`
	Quantity       int  `json:"quantity" validate:"required,gte=1,lte=999"`
}

type UpdateCartItemDTO struct {
	Quantity int `json:"quantity" validate:"required,gte=1,lte=999"`
}

// CartItemResponseDTO: harga selalu dibaca live dari product_models (seperti Shopee).
type CartItemResponseDTO struct {
	ID             uint    `json:"id"`
	ProductModelID uint    `json:"product_model_id"`
	ProductID      uint    `json:"product_id"`
	ProductName    string  `json:"product_name"`
	VariationName  string  `json:"variation_name"`
	SKU            *string `json:"sku,omitempty"`
	UnitPrice      int64   `json:"unit_price"`
	OriginalPrice  *int64  `json:"original_price,omitempty"`
	Quantity       int     `json:"quantity"`
	Subtotal       int64   `json:"subtotal"`
	Stock          int     `json:"stock"`
	Available      bool    `json:"available"` // false = model nonaktif / produk tidak dijual / stok kurang
}

type CartResponseDTO struct {
	ID            uint                  `json:"id"`
	Items         []CartItemResponseDTO `json:"items"`
	TotalQuantity int                   `json:"total_quantity"` // hanya item yang available
	TotalPrice    int64                 `json:"total_price"`    // hanya item yang available
}
