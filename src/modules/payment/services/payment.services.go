package services

import (
	"backend/src/config"
	"backend/src/modules/Payment/models"
	"backend/src/repository"
	"context"
	"encoding/json"
	"fmt"
	"time"
	"backend/src/modules/Order/services"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
)

func CreatePayment(input *models.PaymentRequestDTO) (*models.PaymentResponseDTO, error) {
	ctx := context.Background()
	//redis checking
	cacheKey := fmt.Sprintf("payment:%d:%d", input.UserID, input.OrderID)
	cachedPayment, err := config.RedisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		var payment models.PaymentResponseDTO
		if err := json.Unmarshal([]byte(cachedPayment), &payment); err == nil {
			return &payment, nil
		}
	}
	//req payment
	reqPayment := &coreapi.ChargeReq{
		PaymentType: coreapi.CoreapiPaymentType(input.PaymentType),
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  input.OrderRef,
			GrossAmt: int64(input.Amount),
		},
		BankTransfer: &coreapi.BankTransferDetails{
			Bank:     midtrans.Bank(input.PaymentDetail["bank"].(string)),
			VaNumber: input.PaymentDetail["va_number"].(string),
		},
	}

	resp, err := config.MidtransClient.ChargeTransaction(reqPayment)
	if err != nil {
		return nil, fmt.Errorf("failed to charge transaction: %v", err)
	}

	paymentDetailJSON, err := json.Marshal(input.PaymentDetail)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment detail: %v", err)
	}

	//save to db
	paymentResp := repository.Payment{
		UserID:            input.UserID,
		OrderID:           input.OrderID,
		TransactionID:     resp.TransactionID,
		PaymentType:       input.PaymentType,
		Amount:            input.Amount,
		OrderRef:          input.OrderRef,
		PaymentDetail:     paymentDetailJSON,
		FraudStatus:       resp.FraudStatus,
		TransactionStatus: resp.TransactionStatus,
	}
	if err := config.DB.Create(&paymentResp).Error; err != nil {
		return nil, fmt.Errorf("failed to create payment record: %v", err)
	}

	//cache
	data, _ := json.Marshal(paymentResp)
	config.RedisClient.Set(ctx, cacheKey, data, 10*time.Minute).Err()

	var paymentDetailMap map[string]interface{}
	if err := json.Unmarshal(paymentResp.PaymentDetail, &paymentDetailMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payment detail: %v", err)
	}

	return &models.PaymentResponseDTO{
		ID:                paymentResp.ID,
		UserID:            paymentResp.UserID,
		OrderID:           paymentResp.OrderID,
		TransactionID:     paymentResp.TransactionID,
		PaymentType:       paymentResp.PaymentType,
		Amount:            paymentResp.Amount,
		OrderRef:          paymentResp.OrderRef,
		PaymentDetail:     paymentDetailMap,
		FraudStatus:       paymentResp.FraudStatus,
		TransactionStatus: paymentResp.TransactionStatus,
	}, nil
}

func GetPayment(userID uint, orderID uint) (*models.PaymentResponseDTO, error) {
	ctx := context.Background()
	cachedKey := fmt.Sprintf("payment:%d:%d", userID, orderID)
	cachedPayment, err := config.RedisClient.Get(ctx, cachedKey).Result()
	if err == nil {
		var payment models.PaymentResponseDTO
		if err := json.Unmarshal([]byte(cachedPayment), &payment); err == nil {
			return &payment, nil
		}
	}

	// If not found in cache, retrieve from database
	var paymentRecord repository.Payment
	if err := config.DB.Where("user_id = ? AND order_id = ?", userID, orderID).First(&paymentRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to get payment record: %v", err)
	}

	var paymentDetailMap map[string]interface{}
	if err := json.Unmarshal(paymentRecord.PaymentDetail, &paymentDetailMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payment detail: %v", err)
	}

	return &models.PaymentResponseDTO{
		ID:                paymentRecord.ID,
		UserID:            paymentRecord.UserID,
		OrderID:           paymentRecord.OrderID,
		TransactionID:     paymentRecord.TransactionID,
		PaymentType:       paymentRecord.PaymentType,
		Amount:            paymentRecord.Amount,
		OrderRef:          paymentRecord.OrderRef,
		PaymentDetail:     paymentDetailMap,
		FraudStatus:       paymentRecord.FraudStatus,
		TransactionStatus: paymentRecord.TransactionStatus,
	}, nil
}

func HandleNotification(notificationPayload map[string]interface{}) (*models.PaymentResponseDTO, error) {
	orderID, ok := notificationPayload["order_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid order_id in notification payload")
	}

	// Get Transaction From Midtrans
	resp, err := config.MidtransClient.CheckTransaction(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to check transaction: %v", err)
	}

	// Update status pembayaran
	var paymentRecord repository.Payment
	if err := config.DB.Where("order_id = ?", orderID).First(&paymentRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to get payment record: %v", err)
	}

	paymentRecord.TransactionStatus = resp.TransactionStatus
	paymentRecord.FraudStatus = resp.FraudStatus
	if err := config.DB.Save(&paymentRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to update payment record: %v", err)
	}

	ctx := context.Background()
	cacheKey := fmt.Sprintf("payment:%d:%d", paymentRecord.UserID, paymentRecord.OrderID)

	var paymentDetailMap map[string]interface{}
	if err := json.Unmarshal(paymentRecord.PaymentDetail, &paymentDetailMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payment detail: %v", err)
	}
	paymentResp := &models.PaymentResponseDTO{
		ID:                paymentRecord.ID,
		UserID:            paymentRecord.UserID,
		OrderID:           paymentRecord.OrderID,
		TransactionID:     paymentRecord.TransactionID,
		PaymentType:       paymentRecord.PaymentType,
		Amount:            paymentRecord.Amount,
		OrderRef:          paymentRecord.OrderRef,
		PaymentDetail:     paymentDetailMap,
		FraudStatus:       paymentRecord.FraudStatus,
		TransactionStatus: paymentRecord.TransactionStatus,
	}
	if cacheJSON, err := json.Marshal(paymentResp); err == nil {
		config.RedisClient.Set(ctx, cacheKey, cacheJSON, 10*time.Minute)
	}

	if paymentRecord.TransactionStatus == "settlement" {
		if err := services.UpdateOrderStatus(paymentRecord.OrderID, "paid"); err != nil {
			return nil, fmt.Errorf("failed to update order status: %v", err)
		}
	}


	return paymentResp, nil
}

