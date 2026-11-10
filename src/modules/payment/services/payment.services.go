// Package services: pembayaran lewat Xendit Payment Sessions (hosted checkout).
//
// Alur: POST /orders (checkout) → POST /orders/:id/pay → redirect ke checkout_url
// → webhook Xendit → status dicek ulang lewat GET /sessions/{id} → order PAID.
// Isi webhook TIDAK pernah dipercaya; status PAID hanya dari GetSession.
package services

import (
	"backend/src/config"
	orderservices "backend/src/modules/order/services"
	"backend/src/modules/payment/models"
	"backend/src/modules/payment/xendit"
	"backend/src/repository"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrValidation    = errors.New("validation error")
	ErrConflict      = errors.New("conflict")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrGateway       = errors.New("payment gateway error")
	ErrNotConfigured = errors.New("pembayaran belum dikonfigurasi")
	// ErrRetryLater: webhook belum bisa diproses (mis. status sesi di Xendit belum
	// sesuai event) → dijawab 503 supaya Xendit mengirim ulang.
	ErrRetryLater = errors.New("retry later")
)

// Gateway = bagian client Xendit yang dipakai service (diganti server palsu saat test).
type Gateway interface {
	CreateSession(ctx context.Context, req xendit.CreateSessionRequest) (*xendit.Session, error)
	GetSession(ctx context.Context, id string) (*xendit.Session, error)
	CancelSession(ctx context.Context, id string) (*xendit.Session, error)
}

type Settings struct {
	WebhookToken  string        // dibandingkan dengan header x-callback-token
	SuccessURL    string        // HTTPS, halaman FE "memverifikasi pembayaran"
	CancelURL     string        // HTTPS
	MaxSessionTTL time.Duration // batas atas expires_at sesi (default 24 jam)
}

var (
	gateway  Gateway
	settings Settings
)

// Configure memasang gateway & pengaturan (dipanggil main.go atau test).
func Configure(g Gateway, s Settings) {
	if s.MaxSessionTTL <= 0 {
		s.MaxSessionTTL = 24 * time.Hour
	}
	gateway, settings = g, s
}

// ConfigureFromEnv membaca XENDIT_SECRET_KEY, XENDIT_WEBHOOK_TOKEN, XENDIT_API_URL
// (opsional), XENDIT_SESSION_MAX_TTL (opsional, mis. "24h"), CHECKOUT_SUCCESS_URL,
// CHECKOUT_CANCEL_URL. Kalau ada yang kosong/salah, pembayaran dimatikan
// (endpoint bayar & webhook menjawab 503) dan error dikembalikan untuk di-log.
func ConfigureFromEnv() error {
	var missing []string
	get := func(k string) string {
		v := strings.TrimSpace(os.Getenv(k))
		if v == "" {
			missing = append(missing, k)
		}
		return v
	}
	key, token := get("XENDIT_SECRET_KEY"), get("XENDIT_WEBHOOK_TOKEN")
	success, cancel := get("CHECKOUT_SUCCESS_URL"), get("CHECKOUT_CANCEL_URL")
	if len(missing) > 0 {
		gateway = nil
		return fmt.Errorf("%w: env kosong: %s", ErrNotConfigured, strings.Join(missing, ", "))
	}
	for k, v := range map[string]string{"CHECKOUT_SUCCESS_URL": success, "CHECKOUT_CANCEL_URL": cancel} {
		if u, err := url.Parse(v); err != nil || u.Scheme != "https" || u.Host == "" {
			gateway = nil
			return fmt.Errorf("%w: %s harus URL HTTPS (syarat Xendit)", ErrNotConfigured, k)
		}
	}
	ttl := 24 * time.Hour
	if v := os.Getenv("XENDIT_SESSION_MAX_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 15*time.Minute {
			gateway = nil
			return fmt.Errorf("%w: XENDIT_SESSION_MAX_TTL %q tidak valid (mis. 24h, minimal 15m)", ErrNotConfigured, v)
		}
		ttl = d
	}
	Configure(xendit.New(os.Getenv("XENDIT_API_URL"), key),
		Settings{WebhookToken: token, SuccessURL: success, CancelURL: cancel, MaxSessionTTL: ttl})
	return nil
}

func configured() bool { return gateway != nil }

const (
	// Payment PENDING tanpa checkout_url yang lebih baru dari ini dianggap masih
	// dibuat oleh request lain (jangan buat sesi kedua).
	pendingCreateGrace = 30 * time.Second
	// Xendit: expires_at minimal 10 menit dari sekarang.
	minSessionTTL = 11 * time.Minute
	// Rekonsiliasi: payment PENDING lebih tua dari ini dicek ke Xendit (webhook terlewat).
	reconcileAfter = 10 * time.Minute
)

// PayOrder membuat (atau memakai ulang) sesi pembayaran Xendit untuk order milik user.
// Idempoten: selama ada payment PENDING yang belum kedaluwarsa, checkout_url yang
// sama dikembalikan.
func PayOrder(ctx context.Context, userID, orderID uint) (*models.PaymentResponseDTO, error) {
	if !configured() {
		return nil, ErrNotConfigured
	}
	now := time.Now()
	var (
		pay   repository.Payment
		order *repository.Order
		reuse bool
	)
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		if order, err = orderservices.LockOrder(tx, orderID, &userID); err != nil {
			if errors.Is(err, orderservices.ErrNotFound) {
				return fmt.Errorf("order %d: %w", orderID, ErrNotFound)
			}
			return err
		}
		if order.Status != repository.OrderPendingPayment {
			return fmt.Errorf("%w: order berstatus %s", ErrValidation, order.Status)
		}
		if order.ExpiresAt == nil || order.ExpiresAt.Sub(now) < minSessionTTL {
			return fmt.Errorf("%w: batas waktu pembayaran order ini hampir/sudah habis, buat order baru", ErrValidation)
		}
		var existing repository.Payment
		err = tx.Where("order_id = ? AND status = ?", order.ID, repository.PaymentPending).First(&existing).Error
		switch {
		case err == nil && existing.CheckoutURL != nil && (existing.ExpiresAt == nil || existing.ExpiresAt.After(now.Add(time.Minute))):
			pay, reuse = existing, true
			return nil
		case err == nil && existing.CheckoutURL == nil && now.Sub(existing.CreatedAt) < pendingCreateGrace:
			return fmt.Errorf("%w: pembayaran sedang dibuat, coba lagi sebentar", ErrConflict)
		case err == nil:
			// percobaan lama yang terputus / hampir kedaluwarsa → ditutup, buat baru
			reason := "digantikan percobaan pembayaran baru"
			if err := tx.Model(&existing).Updates(map[string]any{"status": repository.PaymentCancelled, "failure_reason": reason}).Error; err != nil {
				return err
			}
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}
		expires := sessionExpiry(now, *order.ExpiresAt)
		pay = repository.Payment{OrderID: order.ID, Provider: repository.PaymentProviderXendit, ReferenceID: uuid.NewString(),
			Amount: order.Total, Currency: "IDR", Status: repository.PaymentPending, ExpiresAt: &expires}
		return tx.Create(&pay).Error
	})
	if err != nil {
		return nil, err
	}
	if reuse {
		return toResponse(pay), nil
	}
	// Panggilan HTTP di LUAR transaksi DB.
	var items []repository.OrderItem
	if err := config.DB.Where("order_id = ?", order.ID).Order("id").Find(&items).Error; err != nil {
		return nil, err
	}
	sess, gwErr := gateway.CreateSession(ctx, buildSessionRequest(order, &pay, items))
	if gwErr != nil {
		reason := truncate(gwErr.Error(), 500)
		config.DB.Model(&pay).Updates(map[string]any{"status": repository.PaymentFailed, "failure_reason": reason})
		log.Printf("xendit create session order %s: %v", order.OrderNo, gwErr)
		return nil, fmt.Errorf("%w: gagal membuat sesi pembayaran, coba lagi", ErrGateway)
	}
	upd := map[string]any{"provider_session_id": sess.PaymentSessionID, "checkout_url": sess.PaymentLinkURL,
		"provider_response": datatypes.JSON(sess.Raw)}
	if sess.PaymentRequestID != "" {
		upd["provider_payment_request_id"] = sess.PaymentRequestID
	}
	if sess.ExpiresAt != nil {
		upd["expires_at"] = *sess.ExpiresAt
	}
	if err := config.DB.Model(&pay).Updates(upd).Error; err != nil {
		return nil, err
	}
	if err := config.DB.First(&pay, pay.ID).Error; err != nil {
		return nil, err
	}
	return toResponse(pay), nil
}

// sessionExpiry: batas sesi = batas bayar order, dibatasi MaxSessionTTL.
func sessionExpiry(now, orderExpiry time.Time) time.Time {
	exp := orderExpiry
	if limit := now.Add(settings.MaxSessionTTL); exp.After(limit) {
		exp = limit
	}
	return exp.UTC().Truncate(time.Second)
}

func buildSessionRequest(order *repository.Order, pay *repository.Payment, items []repository.OrderItem) xendit.CreateSessionRequest {
	req := xendit.CreateSessionRequest{
		ReferenceID: pay.ReferenceID, Currency: "IDR", Amount: pay.Amount, Country: "ID",
		SessionType: "PAY", Mode: "PAYMENT_LINK", ExpiresAt: pay.ExpiresAt,
		SuccessReturnURL: withOrderParam(settings.SuccessURL, order.OrderNo),
		CancelReturnURL:  withOrderParam(settings.CancelURL, order.OrderNo),
		Description:      "Pesanan Station Parfume " + order.OrderNo,
		Metadata:         map[string]string{"order_no": order.OrderNo},
	}
	// Rincian item hanya dikirim kalau jumlahnya persis sama dengan total
	// (ongkir & diskon sekarang 0, jadi biasanya sama).
	var sum int64
	list := make([]xendit.Item, 0, len(items))
	for _, it := range items {
		name := it.ProductName
		if it.VariantName != nil && *it.VariantName != "" {
			name += " - " + *it.VariantName
		}
		list = append(list, xendit.Item{ReferenceID: truncate(it.VariantCode, 64), Type: "PHYSICAL_PRODUCT", Name: truncate(name, 255),
			NetUnitAmount: it.UnitPrice, Quantity: it.Quantity, Category: "Parfum"})
		sum += it.Subtotal
	}
	if sum == pay.Amount {
		req.Items = list
	}
	return req
}

func withOrderParam(base, orderNo string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("order", orderNo)
	u.RawQuery = q.Encode()
	return u.String()
}

// HandleWebhook memproses webhook Xendit. Token diverifikasi constant-time, event
// mentah disimpan (dedup), lalu status sesi dicek ulang lewat GetSession.
// Error yang dikembalikan → handler menjawab non-2xx supaya Xendit mengirim ulang.
func HandleWebhook(ctx context.Context, token string, body []byte) error {
	if !configured() || settings.WebhookToken == "" {
		return ErrNotConfigured
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(settings.WebhookToken)) != 1 {
		return ErrUnauthorized
	}
	var ev xendit.WebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil || ev.Event == "" || ev.Data.PaymentSessionID == "" {
		return fmt.Errorf("%w: body webhook tidak dikenali", ErrValidation)
	}
	row := repository.PaymentEvent{Provider: repository.PaymentProviderXendit, EventType: ev.Event,
		ProviderPaymentID: ev.Data.PaymentSessionID, Payload: datatypes.JSON(body)}
	if err := config.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return err
	}
	if row.ID == 0 { // sudah pernah diterima
		if err := config.DB.Where("provider = ? AND event_type = ? AND provider_payment_id = ?",
			row.Provider, row.EventType, row.ProviderPaymentID).First(&row).Error; err != nil {
			return err
		}
		if row.ProcessedAt != nil {
			return nil // duplikat yang sudah diproses → no-op
		}
	}
	want := map[string]string{xendit.EventSessionCompleted: xendit.StatusCompleted, xendit.EventSessionExpired: xendit.StatusExpired}[ev.Event]
	if want == "" {
		now := time.Now()
		return config.DB.Model(&row).Update("processed_at", now).Error // event lain: disimpan saja
	}
	paymentID, status, err := syncSession(ctx, ev.Data.PaymentSessionID)
	if err == nil && status != want {
		// Status di Xendit belum sesuai event → jangan tandai selesai; biarkan Xendit
		// mengirim ulang (rekonsiliasi berkala juga akan mengecek).
		err = fmt.Errorf("%w: event %s tapi status sesi %s", ErrRetryLater, ev.Event, status)
	}
	upd := map[string]any{}
	if paymentID != 0 {
		upd["payment_id"] = paymentID
	}
	if err != nil {
		upd["process_error"] = truncate(err.Error(), 1000)
	} else {
		upd["processed_at"] = time.Now()
		upd["process_error"] = nil
	}
	if dbErr := config.DB.Model(&row).Updates(upd).Error; dbErr != nil && err == nil {
		err = dbErr
	}
	return err
}

// SyncSession mengambil status sesi dari Xendit (sumber kebenaran) lalu
// menerapkannya ke payment + order. Mengembalikan id payment kita.
func SyncSession(ctx context.Context, sessionID string) (uint, error) {
	id, _, err := syncSession(ctx, sessionID)
	return id, err
}

func syncSession(ctx context.Context, sessionID string) (uint, string, error) {
	if !configured() {
		return 0, "", ErrNotConfigured
	}
	sess, err := gateway.GetSession(ctx, sessionID)
	if err != nil {
		return 0, "", fmt.Errorf("%w: get session %s: %v", ErrGateway, sessionID, err)
	}
	id, err := applySession(sess)
	return id, sess.Status, err
}

func applySession(sess *xendit.Session) (uint, error) {
	var paymentID uint
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Payment
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("provider = ? AND provider_session_id = ?", repository.PaymentProviderXendit, sess.PaymentSessionID).First(&p).Error
		if errors.Is(err, gorm.ErrRecordNotFound) && sess.ReferenceID != "" {
			// sesi dibuat di Xendit tapi update kita sempat gagal → cari lewat reference_id
			err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("provider = ? AND reference_id = ?", repository.PaymentProviderXendit, sess.ReferenceID).First(&p).Error
			if err == nil {
				if err := tx.Model(&p).Update("provider_session_id", sess.PaymentSessionID).Error; err != nil {
					return err
				}
			}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("payment untuk sesi %s: %w", sess.PaymentSessionID, ErrNotFound)
		}
		if err != nil {
			return err
		}
		paymentID = p.ID
		if sess.ReferenceID != p.ReferenceID {
			return markFailed(tx, &p, fmt.Sprintf("reference_id sesi %q ≠ %q", sess.ReferenceID, p.ReferenceID))
		}
		switch sess.Status {
		case xendit.StatusCompleted:
			return applyCompleted(tx, &p, sess)
		case xendit.StatusExpired:
			return closePending(tx, &p, repository.PaymentExpired)
		case xendit.StatusCanceled:
			return closePending(tx, &p, repository.PaymentCancelled)
		}
		return nil // ACTIVE: belum ada perubahan
	})
	return paymentID, err
}

func applyCompleted(tx *gorm.DB, p *repository.Payment, sess *xendit.Session) error {
	if p.Status == repository.PaymentPaid {
		return nil // idempoten
	}
	amount, err := sess.AmountRupiah()
	if err != nil || amount != p.Amount || (sess.Currency != "" && sess.Currency != "IDR") {
		return markFailed(tx, p, fmt.Sprintf("nominal sesi %s %s ≠ %d IDR", sess.Amount, sess.Currency, p.Amount))
	}
	order, err := orderservices.LockOrder(tx, p.OrderID, nil)
	if err != nil {
		return err
	}
	var paidCount int64
	if err := tx.Model(&repository.Payment{}).Where("order_id = ? AND status = ? AND id <> ?", p.OrderID, repository.PaymentPaid, p.ID).
		Count(&paidCount).Error; err != nil {
		return err
	}
	if paidCount > 0 {
		return markFailed(tx, p, "pembayaran ganda untuk order yang sudah lunas — refund manual lewat Dashboard Xendit")
	}
	now := time.Now()
	upd := map[string]any{"status": repository.PaymentPaid, "paid_at": now, "failure_reason": nil}
	if sess.PaymentID != "" {
		upd["provider_payment_id"] = sess.PaymentID
	}
	if sess.PaymentRequestID != "" {
		upd["provider_payment_request_id"] = sess.PaymentRequestID
	}
	if err := tx.Model(p).Updates(upd).Error; err != nil {
		return err
	}
	switch order.Status {
	case repository.OrderPendingPayment:
		return orderservices.ApplyStatus(tx, order, repository.OrderPaid, nil)
	case repository.OrderExpired, repository.OrderCancelled:
		// dibayar tepat saat/sesudah batas waktu: uang sudah masuk → order tetap diproses
		prev := order.Status
		if err := orderservices.ApplyLatePayment(tx, order); err != nil {
			return err
		}
		log.Printf("pembayaran telat: order %s %s → PAID", order.OrderNo, prev)
		changes, _ := json.Marshal(map[string]any{"status": []string{prev, repository.OrderPaid}, "payment_id": p.ID})
		eid := order.ID
		return tx.Create(&repository.AuditLog{Action: "order.late_payment", Entity: "order", EntityID: &eid, Changes: changes}).Error
	}
	return nil // order sudah PAID/diproses (mis. ditandai manual oleh admin)
}

func closePending(tx *gorm.DB, p *repository.Payment, status string) error {
	if p.Status != repository.PaymentPending {
		return nil
	}
	return tx.Model(p).Update("status", status).Error
}

func markFailed(tx *gorm.DB, p *repository.Payment, reason string) error {
	log.Printf("payment %d (%s) FAILED: %s", p.ID, p.ReferenceID, reason)
	if p.Status == repository.PaymentPaid {
		return nil
	}
	return tx.Model(p).Updates(map[string]any{"status": repository.PaymentFailed, "failure_reason": truncate(reason, 500)}).Error
}

// CancelOrderSessions membatalkan sesi Xendit yang masih aktif untuk order (mis.
// setelah pembeli membatalkan order), supaya link bayarnya mati. Best effort.
func CancelOrderSessions(ctx context.Context, orderID uint) {
	if !configured() {
		return
	}
	var pending []repository.Payment
	if err := config.DB.Where("order_id = ? AND status = ? AND provider = ? AND provider_session_id IS NOT NULL",
		orderID, repository.PaymentPending, repository.PaymentProviderXendit).Find(&pending).Error; err != nil {
		log.Printf("cancel sesi order %d: %v", orderID, err)
		return
	}
	for _, p := range pending {
		if err := cancelSession(ctx, p); err != nil {
			log.Printf("cancel sesi %s: %v", *p.ProviderSessionID, err)
		}
	}
}

// cancelSession: batalkan sesi di Xendit lalu sinkronkan statusnya. Kalau sesi
// ternyata sudah COMPLETED, pembayarannya diterapkan (bukan dibatalkan).
func cancelSession(ctx context.Context, p repository.Payment) error {
	sess, err := gateway.CancelSession(ctx, *p.ProviderSessionID)
	var apiErr *xendit.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		// mis. 422 INVALID_SESSION_STATUS: sesi sudah tidak aktif → ambil status sebenarnya
		sess, err = gateway.GetSession(ctx, *p.ProviderSessionID)
	}
	if err != nil {
		return err
	}
	_, err = applySession(sess)
	return err
}

// ReconcileAndExpire dipanggil berkala dari main.go:
//  1. payment PENDING yang > 10 menit dicek ke Xendit (webhook terlewat);
//  2. order yang lewat batas bayar: sesi dicek & dibatalkan dulu, baru order EXPIRED.
//     Kalau pembatalan sesi gagal karena jaringan, order dilewati sampai putaran berikutnya
//     (supaya tidak ada link bayar yang masih hidup untuk order EXPIRED).
func ReconcileAndExpire(ctx context.Context, now time.Time) (expired int, err error) {
	if !configured() {
		return orderservices.ExpireOverdueOrders(now)
	}
	var stale []repository.Payment
	if err := config.DB.Where("provider = ? AND status = ? AND provider_session_id IS NOT NULL AND created_at < ?",
		repository.PaymentProviderXendit, repository.PaymentPending, now.Add(-reconcileAfter)).Order("id").Find(&stale).Error; err != nil {
		return 0, err
	}
	for _, p := range stale {
		if _, err := SyncSession(ctx, *p.ProviderSessionID); err != nil {
			log.Printf("rekonsiliasi payment %d: %v", p.ID, err)
		}
	}
	ids, err := orderservices.OverdueOrderIDs(now)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		var pending []repository.Payment
		if err := config.DB.Where("order_id = ? AND status = ? AND provider = ? AND provider_session_id IS NOT NULL",
			id, repository.PaymentPending, repository.PaymentProviderXendit).Find(&pending).Error; err != nil {
			return expired, err
		}
		ok := true
		for _, p := range pending {
			if err := cancelSession(ctx, p); err != nil {
				log.Printf("expire order %d: batalkan sesi %s gagal, dicoba lagi nanti: %v", id, *p.ProviderSessionID, err)
				ok = false
			}
		}
		if !ok {
			continue
		}
		// percobaan yang sesinya tidak pernah jadi (create gagal di tengah jalan)
		if err := config.DB.Model(&repository.Payment{}).
			Where("order_id = ? AND status = ? AND provider_session_id IS NULL", id, repository.PaymentPending).
			Updates(map[string]any{"status": repository.PaymentCancelled, "failure_reason": "order kedaluwarsa"}).Error; err != nil {
			return expired, err
		}
		done, err := orderservices.ExpireOrder(id) // false kalau ternyata sudah PAID lewat sesi di atas
		if err != nil {
			return expired, err
		}
		if done {
			expired++
		}
	}
	return expired, nil
}

func toResponse(p repository.Payment) *models.PaymentResponseDTO {
	return &models.PaymentResponseDTO{ID: p.ID, OrderID: p.OrderID, Provider: p.Provider, ReferenceID: p.ReferenceID, Status: p.Status,
		CheckoutURL: p.CheckoutURL, Amount: p.Amount, Currency: p.Currency, ExpiresAt: p.ExpiresAt, PaidAt: p.PaidAt, CreatedAt: p.CreatedAt}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
