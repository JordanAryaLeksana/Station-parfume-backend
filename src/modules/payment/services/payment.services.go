package services

import (
	"backend/src/config"
	orderservices "backend/src/modules/order/services"
	"backend/src/modules/payment/models"
	"backend/src/repository"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var validate = validator.New()

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

// CreatePayment membuat VA Midtrans untuk order milik userID yang belum dibayar.
// Nominal diambil dari orders.total. Kalau sudah ada payment PENDING untuk order
// itu, payment tersebut dikembalikan (tidak membuat VA baru).
func CreatePayment(userID uint, input *models.PaymentRequestDTO) (*models.PaymentResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	var order repository.Order
	if err := config.DB.Where("id = ? AND user_id = ?", input.OrderID, userID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("order %d: %w", input.OrderID, ErrNotFound)
		}
		return nil, err
	}
	if order.Status != repository.OrderPendingPayment {
		return nil, fmt.Errorf("%w: order berstatus %s", ErrValidation, order.Status)
	}
	var pending repository.Payment
	err := config.DB.Where("order_id = ? AND status = ?", order.ID, repository.PaymentPending).First(&pending).Error
	if err == nil {
		return toResponse(pending), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	ref := uuid.NewString()
	req := &coreapi.ChargeReq{
		PaymentType: coreapi.CoreapiPaymentType(input.PaymentType),
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  ref, // order_id di Midtrans = reference_id kita
			GrossAmt: order.Total,
		},
		BankTransfer: &coreapi.BankTransferDetails{Bank: midtrans.Bank(input.Bank)},
	}
	resp, mErr := config.MidtransClient.ChargeTransaction(req)
	if mErr != nil { // mErr bertipe *midtrans.Error, jadi cek nil ini benar (bukan interface error)
		return nil, fmt.Errorf("failed to charge transaction: %s", mErr.GetMessage())
	}
	raw, _ := json.Marshal(resp)
	payment := repository.Payment{
		OrderID:           order.ID,
		Provider:          repository.PaymentProviderMidtrans,
		ReferenceID:       ref,
		ProviderPaymentID: optionalString(resp.TransactionID),
		PaymentMethod:     optionalString(input.PaymentType),
		PaymentChannel:    optionalString(input.Bank),
		Amount:            order.Total,
		Currency:          "IDR",
		Status:            repository.PaymentPending,
		ExpiresAt:         parseMidtransTime(resp.ExpiryTime),
		ProviderResponse:  datatypes.JSON(raw),
	}
	if err := config.DB.Create(&payment).Error; err != nil {
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}
	return toResponse(payment), nil
}

// GetPayment hanya mengembalikan payment dari order milik userID.
func GetPayment(paymentID, userID uint) (*models.PaymentResponseDTO, error) {
	var p repository.Payment
	err := config.DB.
		Joins("JOIN orders ON orders.id = payments.order_id").
		Where("payments.id = ? AND orders.user_id = ?", paymentID, userID).
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("payment %d: %w", paymentID, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return toResponse(p), nil
}

// HandleNotification: status TIDAK dipercaya dari payload; selalu dicek ulang
// ke Midtrans (CheckTransaction), lalu payment + order diperbarui dalam satu transaksi.
func HandleNotification(payload map[string]any) (*models.PaymentResponseDTO, error) {
	ref, ok := payload["order_id"].(string)
	if !ok || ref == "" {
		return nil, fmt.Errorf("%w: order_id tidak ada di payload", ErrValidation)
	}
	if _, err := uuid.Parse(ref); err != nil {
		return nil, fmt.Errorf("%w: order_id bukan reference_id yang valid", ErrValidation)
	}
	resp, mErr := config.MidtransClient.CheckTransaction(ref)
	if mErr != nil {
		return nil, fmt.Errorf("failed to check transaction: %s", mErr.GetMessage())
	}
	newStatus := mapMidtransStatus(resp.TransactionStatus, resp.FraudStatus)

	var result repository.Payment
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference_id = ? AND provider = ?", ref, repository.PaymentProviderMidtrans).
			First(&p).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("payment %s: %w", ref, ErrNotFound)
			}
			return err
		}
		result = p
		if p.Status != repository.PaymentPending || newStatus == repository.PaymentPending {
			return nil // sudah final atau belum berubah → idempoten
		}
		if newStatus == repository.PaymentPaid && !amountMatches(resp.GrossAmount, p.Amount) {
			newStatus = repository.PaymentFailed
			reason := fmt.Sprintf("gross_amount %s tidak sama dengan amount %d", resp.GrossAmount, p.Amount)
			p.FailureReason = &reason
		}
		updates := map[string]any{"status": newStatus, "failure_reason": p.FailureReason}
		if resp.TransactionID != "" {
			updates["provider_payment_id"] = resp.TransactionID
		}
		now := time.Now()
		if newStatus == repository.PaymentPaid {
			updates["paid_at"] = now
		}
		if err := tx.Model(&p).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update payment: %w", err)
		}

		order, err := orderservices.LockOrder(tx, p.OrderID, nil)
		if err != nil {
			return err
		}
		if order.Status == repository.OrderPendingPayment {
			switch newStatus {
			case repository.PaymentPaid:
				err = orderservices.ApplyStatus(tx, order, repository.OrderPaid, nil)
			case repository.PaymentExpired:
				reason := "pembayaran kedaluwarsa"
				err = orderservices.ApplyStatus(tx, order, repository.OrderExpired, &reason)
			}
			if err != nil {
				return err
			}
		}
		return tx.First(&result, p.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return toResponse(result), nil
}

func mapMidtransStatus(txStatus, fraud string) string {
	switch txStatus {
	case "settlement":
		return repository.PaymentPaid
	case "capture":
		if fraud == "" || fraud == "accept" {
			return repository.PaymentPaid
		}
		return repository.PaymentPending // challenge: tunggu keputusan
	case "expire":
		return repository.PaymentExpired
	case "cancel":
		return repository.PaymentCancelled
	case "deny", "failure":
		return repository.PaymentFailed
	case "refund", "partial_refund":
		return repository.PaymentRefunded
	default:
		return repository.PaymentPending
	}
}

// amountMatches membandingkan gross_amount Midtrans ("45000.00") dengan amount rupiah.
func amountMatches(gross string, amount int64) bool {
	whole, frac, _ := strings.Cut(gross, ".")
	if strings.Trim(frac, "0") != "" {
		return false
	}
	v, err := strconv.ParseInt(whole, 10, 64)
	return err == nil && v == amount
}

func parseMidtransTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc)
	if err != nil {
		return nil
	}
	return &t
}

func toResponse(p repository.Payment) *models.PaymentResponseDTO {
	dto := &models.PaymentResponseDTO{
		ID: p.ID, OrderID: p.OrderID, Provider: p.Provider, ReferenceID: p.ReferenceID,
		ProviderPaymentID: p.ProviderPaymentID, PaymentMethod: p.PaymentMethod, PaymentChannel: p.PaymentChannel,
		Amount: p.Amount, Currency: p.Currency, Status: p.Status, ExpiresAt: p.ExpiresAt, PaidAt: p.PaidAt, CreatedAt: p.CreatedAt,
	}
	var raw struct {
		VaNumbers       []models.VANumberDTO `json:"va_numbers"`
		PermataVaNumber string               `json:"permata_va_number"`
	}
	if len(p.ProviderResponse) > 0 && json.Unmarshal(p.ProviderResponse, &raw) == nil {
		dto.VANumbers = raw.VaNumbers
		if raw.PermataVaNumber != "" {
			dto.VANumbers = append(dto.VANumbers, models.VANumberDTO{Bank: "permata", VANumber: raw.PermataVaNumber})
		}
	}
	return dto
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
