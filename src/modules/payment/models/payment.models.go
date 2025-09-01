package models


type PaymentRequestDTO struct {
    UserID        uint    `json:"user_id"`
    OrderID       uint    `json:"order_id"`
    TransactionID string  `json:"transaction_id"`
    PaymentType   string  `json:"payment_type"` // e.g. bank_transfer, qris, gopay
    Amount        float64 `json:"amount"`
    OrderRef      string  `json:"order_ref"`

    PaymentDetail map[string]interface{} `json:"payment_detail"`
}


type PaymentResponseDTO struct {
    ID                uint                   `json:"id"`
    UserID            uint                   `json:"user_id"`
    OrderID           uint                   `json:"order_id"`
    TransactionID     string                 `json:"transaction_id"`
    PaymentType       string                 `json:"payment_type"`
    Amount            float64                `json:"amount"`
    OrderRef          string                 `json:"order_ref"`
    PaymentDetail     map[string]interface{} `json:"payment_detail"`
    FraudStatus       string                 `json:"fraud_status"`
    TransactionStatus string                 `json:"transaction_status"`
}
