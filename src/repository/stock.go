package repository

import "time"

// ---------------------------------------------------------------------
// Riwayat stok. Stoknya sendiri ada di Variant.Stock / Variant.Reserved
// (hanya untuk produk track_stock = true). Setiap perubahan Variant.Stock
// dicatat di sini (append-only).
// ---------------------------------------------------------------------

const (
	MovementInitial    = "initial"
	MovementRestock    = "restock"
	MovementSale       = "sale"
	MovementReturn     = "return"
	MovementAdjustment = "adjustment"
	MovementOpname     = "opname"
)

type StockMovement struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	VariantID  uint      `gorm:"not null" json:"variant_id"`
	Delta      int       `gorm:"not null" json:"delta"`
	StockAfter int       `gorm:"not null" json:"stock_after"`
	Reason     string    `gorm:"size:20;not null" json:"reason"`
	OrderID    *uint     `json:"order_id,omitempty"`
	UserID     *uint     `json:"user_id,omitempty"`
	Note       *string   `json:"note,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
