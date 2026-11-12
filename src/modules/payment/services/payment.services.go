// Package services: pembayaran lewat Midtrans Snap (hosted checkout).
//
// Alur: POST /orders (checkout) → POST /orders/:id/pay → redirect ke checkout_url
// (halaman Snap) → notifikasi Midtrans → status dicek ulang lewat GET status → order PAID.
// Isi notifikasi TIDAK pernah dipercaya; status PAID hanya dari GetStatus.
//
// Satu baris payments = satu transaksi Snap: payments.reference_id (UUID) dikirim
// sebagai order_id Midtrans, provider_session_id = Snap token, checkout_url =
// redirect_url, provider_payment_id = transaction_id (diisi setelah pembeli
// memilih metode bayar).
package services

import (
	"backend/src/config"
	orderservices "backend/src/modules/order/services"
	"backend/src/modules/payment/midtrans"
	"backend/src/modules/payment/models"
	"backend/src/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
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
	// ErrRetryLater: notifikasi belum bisa diproses (mis. status di Midtrans belum
	// sesuai notifikasi) → dijawab 503 supaya Midtrans mengirim ulang.
	ErrRetryLater = errors.New("retry later")
)

// Gateway = bagian client Midtrans yang dipakai service (diganti server palsu saat test).
type Gateway interface {
	CreateSnap(ctx context.Context, req midtrans.SnapRequest) (*midtrans.SnapResponse, error)
	GetStatus(ctx context.Context, id string) (*midtrans.Transaction, error)
	Expire(ctx context.Context, id string) (*midtrans.Transaction, error)
}

type Settings struct {
	ServerKey     string        // untuk memverifikasi signature_key notifikasi
	SuccessURL    string        // halaman FE "memverifikasi pembayaran" (callbacks.finish)
	MaxSessionTTL time.Duration // batas atas umur transaksi Snap (default 24 jam)
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

// ConfigureFromEnv membaca MIDTRANS_SERVER_KEY, CHECKOUT_SUCCESS_URL (wajib),
// MIDTRANS_IS_PRODUCTION (opsional, default false = sandbox), MIDTRANS_SNAP_URL /
// MIDTRANS_API_URL (opsional, override URL), PAYMENT_SESSION_MAX_TTL (opsional,
// mis. "24h"). Kalau ada yang kosong/salah, pembayaran dimatikan (endpoint bayar &
// webhook menjawab 503) dan error dikembalikan untuk di-log.
func ConfigureFromEnv() error {
	fail := func(format string, a ...any) error {
		gateway = nil
		return fmt.Errorf("%w: "+format, append([]any{ErrNotConfigured}, a...)...)
	}
	var missing []string
	get := func(k string) string {
		v := strings.TrimSpace(os.Getenv(k))
		if v == "" {
			missing = append(missing, k)
		}
		return v
	}
	key, success := get("MIDTRANS_SERVER_KEY"), get("CHECKOUT_SUCCESS_URL")
	if len(missing) > 0 {
		return fail("env kosong: %s", strings.Join(missing, ", "))
	}
	if !validReturnURL(success) {
		return fail("CHECKOUT_SUCCESS_URL harus URL HTTPS (http hanya untuk localhost)")
	}
	production := false
	if v := strings.TrimSpace(os.Getenv("MIDTRANS_IS_PRODUCTION")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fail("MIDTRANS_IS_PRODUCTION %q tidak valid (true/false)", v)
		}
		production = b
	}
	// Key sandbox diawali "SB-"; key production tidak. Campur keduanya pasti 401.
	if production && strings.HasPrefix(key, "SB-") {
		return fail("MIDTRANS_IS_PRODUCTION=true tapi MIDTRANS_SERVER_KEY adalah key sandbox (SB-...)")
	}
	if !production && strings.HasPrefix(key, "Mid-server-") {
		return fail("MIDTRANS_SERVER_KEY adalah key production; set MIDTRANS_IS_PRODUCTION=true atau pakai key sandbox")
	}
	ttl := 24 * time.Hour
	if v := os.Getenv("PAYMENT_SESSION_MAX_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 15*time.Minute {
			return fail("PAYMENT_SESSION_MAX_TTL %q tidak valid (mis. 24h, minimal 15m)", v)
		}
		ttl = d
	}
	Configure(midtrans.New(key, production, os.Getenv("MIDTRANS_SNAP_URL"), os.Getenv("MIDTRANS_API_URL")),
		Settings{ServerKey: key, SuccessURL: success, MaxSessionTTL: ttl})
	return nil
}

func validReturnURL(v string) bool {
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return false
	}
	h := u.Hostname()
	return u.Scheme == "https" || (u.Scheme == "http" && (h == "localhost" || h == "127.0.0.1"))
}

func configured() bool { return gateway != nil }

const (
	// Payment PENDING tanpa checkout_url yang lebih baru dari ini dianggap masih
	// dibuat oleh request lain (jangan buat transaksi Snap kedua).
	pendingCreateGrace = 30 * time.Second
	// Sisa waktu bayar minimal supaya transaksi Snap baru masih masuk akal.
	minSessionTTL = 11 * time.Minute
	// Rekonsiliasi: payment PENDING lebih tua dari ini dicek ke Midtrans (notifikasi terlewat).
	reconcileAfter = 10 * time.Minute
)

// PayOrder membuat (atau memakai ulang) transaksi Snap untuk order milik user.
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
		case err == nil && existing.Provider == repository.PaymentProviderMidtrans && existing.CheckoutURL != nil &&
			(existing.ExpiresAt == nil || existing.ExpiresAt.After(now.Add(time.Minute))):
			pay, reuse = existing, true
			return nil
		case err == nil && existing.CheckoutURL == nil && now.Sub(existing.CreatedAt) < pendingCreateGrace:
			return fmt.Errorf("%w: pembayaran sedang dibuat, coba lagi sebentar", ErrConflict)
		case err == nil:
			// percobaan lama yang terputus / hampir kedaluwarsa / dari gateway lama → ditutup, buat baru
			reason := "digantikan percobaan pembayaran baru"
			if err := tx.Model(&existing).Updates(map[string]any{"status": repository.PaymentCancelled, "failure_reason": reason}).Error; err != nil {
				return err
			}
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}
		expires := sessionExpiry(now, *order.ExpiresAt)
		pay = repository.Payment{OrderID: order.ID, Provider: repository.PaymentProviderMidtrans, ReferenceID: uuid.NewString(),
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
	snap, gwErr := gateway.CreateSnap(ctx, buildSnapRequest(order, &pay, items, now))
	if gwErr != nil {
		reason := truncate(gwErr.Error(), 500)
		config.DB.Model(&pay).Updates(map[string]any{"status": repository.PaymentFailed, "failure_reason": reason})
		log.Printf("midtrans create snap order %s: %v", order.OrderNo, gwErr)
		return nil, fmt.Errorf("%w: gagal membuat transaksi pembayaran, coba lagi", ErrGateway)
	}
	upd := map[string]any{"provider_session_id": snap.Token, "checkout_url": snap.RedirectURL,
		"provider_response": datatypes.JSON(snap.Raw)}
	if err := config.DB.Model(&pay).Updates(upd).Error; err != nil {
		return nil, err
	}
	if err := config.DB.First(&pay, pay.ID).Error; err != nil {
		return nil, err
	}
	return toResponse(pay), nil
}

// sessionExpiry: batas transaksi = batas bayar order, dibatasi MaxSessionTTL,
// dibulatkan ke bawah ke menit penuh dari `now` (expiry Midtrans dalam menit).
func sessionExpiry(now, orderExpiry time.Time) time.Time {
	exp := orderExpiry
	if limit := now.Add(settings.MaxSessionTTL); exp.After(limit) {
		exp = limit
	}
	return now.Add(exp.Sub(now).Truncate(time.Minute))
}

func buildSnapRequest(order *repository.Order, pay *repository.Payment, items []repository.OrderItem, now time.Time) midtrans.SnapRequest {
	req := midtrans.SnapRequest{
		TransactionDetails: midtrans.TransactionDetails{OrderID: pay.ReferenceID, GrossAmount: pay.Amount},
		Callbacks:          &midtrans.Callbacks{Finish: withOrderParam(settings.SuccessURL, order.OrderNo)},
		CustomField1:       order.OrderNo,
	}
	if pay.ExpiresAt != nil {
		if minutes := int(pay.ExpiresAt.Sub(now) / time.Minute); minutes > 0 {
			// start_time diisi supaya batas waktu dihitung sejak link dibuat, bukan
			// sejak pembeli memilih metode bayar.
			req.Expiry = &midtrans.Expiry{StartTime: midtrans.FormatTime(now), Unit: "minute", Duration: minutes}
			req.PageExpiry = &midtrans.PageExpiry{Duration: minutes, Unit: "minute"}
		}
	}
	// Rincian item hanya dikirim kalau jumlahnya persis sama dengan total (Midtrans
	// menolak kalau beda; ongkir & diskon sekarang 0, jadi biasanya sama).
	var sum int64
	list := make([]midtrans.Item, 0, len(items))
	for _, it := range items {
		name := it.ProductName
		if it.VariantName != nil && *it.VariantName != "" {
			name += " - " + *it.VariantName
		}
		list = append(list, midtrans.Item{ID: truncate(it.VariantCode, 50), Price: it.UnitPrice, Quantity: it.Quantity,
			Name: truncate(name, 50), Category: "Parfum"})
		sum += it.Subtotal
	}
	if sum == pay.Amount {
		req.ItemDetails = list
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

// finalStatus: notifikasi yang mengubah payment (status lain hanya disimpan).
func finalStatus(s string) bool {
	switch s {
	case midtrans.StatusSettlement, midtrans.StatusCapture, midtrans.StatusDeny, midtrans.StatusCancel,
		midtrans.StatusExpire, midtrans.StatusFailure:
		return true
	}
	return false
}

// HandleWebhook memproses notifikasi Midtrans. signature_key diverifikasi
// (constant-time), notifikasi mentah disimpan (dedup), lalu status dicek ulang
// lewat GetStatus. Error yang dikembalikan → handler menjawab non-2xx supaya
// Midtrans mengirim ulang.
func HandleWebhook(ctx context.Context, body []byte) error {
	if !configured() || settings.ServerKey == "" {
		return ErrNotConfigured
	}
	var n midtrans.Transaction
	if err := json.Unmarshal(body, &n); err != nil || n.OrderID == "" || n.StatusCode == "" || n.GrossAmount == "" || n.TransactionStatus == "" {
		return fmt.Errorf("%w: body notifikasi tidak dikenali", ErrValidation)
	}
	if !midtrans.VerifySignature(&n, settings.ServerKey) {
		return ErrUnauthorized
	}
	status := n.Status()
	eventType := status
	if f := n.Fraud(); f != "" {
		eventType += "/" + f // capture/challenge lalu capture/accept = dua event berbeda
	}
	lookup := n.TransactionID
	if lookup == "" {
		lookup = n.OrderID
	}
	row := repository.PaymentEvent{Provider: repository.PaymentProviderMidtrans, EventType: truncate(eventType, 100),
		ProviderPaymentID: truncate(lookup, 100), Payload: datatypes.JSON(body)}
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
	known, err := knownOrderID(n.OrderID)
	if err != nil {
		return err
	}
	if !known {
		// Bukan transaksi toko ini (mis. tombol "test notification" di Dashboard):
		// dijawab 200 supaya Midtrans tidak mengirim ulang, catatannya tetap disimpan.
		return config.DB.Model(&row).Updates(map[string]any{"processed_at": time.Now(),
			"process_error": "order_id tidak dikenal (bukan transaksi toko ini / notifikasi uji)"}).Error
	}
	if !finalStatus(status) {
		now := time.Now()
		return config.DB.Model(&row).Update("processed_at", now).Error // pending/refund/...: disimpan saja
	}
	// transaction_id dari notifikasi yang signature-nya valid dipakai untuk cek
	// status (DANA hanya bisa dicek dengan transaction_id).
	paymentID, cur, err := syncTransaction(ctx, lookup, n.OrderID)
	if err == nil && (cur == "" || cur == midtrans.StatusPending) {
		// Status di Midtrans belum sesuai notifikasi → jangan tandai selesai; biarkan
		// Midtrans mengirim ulang (rekonsiliasi berkala juga akan mengecek).
		err = fmt.Errorf("%w: notifikasi %s tapi status di Midtrans %q", ErrRetryLater, status, cur)
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

// knownOrderID: order_id Midtrans milik salah satu payment kita (reference_id UUID).
func knownOrderID(id string) (bool, error) {
	if _, err := uuid.Parse(id); err != nil {
		return false, nil
	}
	var n int64
	err := config.DB.Model(&repository.Payment{}).
		Where("provider = ? AND reference_id = ?", repository.PaymentProviderMidtrans, id).Count(&n).Error
	return n > 0, err
}

// gatewayID: id untuk API status/expire. transaction_id kalau sudah diketahui
// (wajib untuk DANA), kalau belum order_id (= reference_id kita).
func gatewayID(p repository.Payment) string {
	if p.ProviderPaymentID != nil && *p.ProviderPaymentID != "" {
		return *p.ProviderPaymentID
	}
	return p.ReferenceID
}

// syncTransaction mengambil status dari Midtrans (sumber kebenaran) lalu
// menerapkannya ke payment + order. orderID = order_id Midtrans yang diharapkan.
// Mengembalikan id payment kita dan transaction_status ("" = transaksi belum ada).
func syncTransaction(ctx context.Context, id, orderID string) (uint, string, error) {
	if !configured() {
		return 0, "", ErrNotConfigured
	}
	t, err := gateway.GetStatus(ctx, id)
	if errors.Is(err, midtrans.ErrTransactionNotFound) {
		return 0, "", nil // pembeli belum memilih metode bayar
	}
	if err != nil {
		return 0, "", fmt.Errorf("%w: status %s: %v", ErrGateway, id, err)
	}
	if orderID != "" && t.OrderID != orderID {
		return 0, t.Status(), fmt.Errorf("%w: order_id status %q ≠ %q", ErrValidation, t.OrderID, orderID)
	}
	pid, err := applyTransaction(t)
	return pid, t.Status(), err
}

func applyTransaction(t *midtrans.Transaction) (uint, error) {
	if _, err := uuid.Parse(t.OrderID); err != nil {
		return 0, fmt.Errorf("payment untuk order_id %q: %w", t.OrderID, ErrNotFound)
	}
	var paymentID uint
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Payment
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("provider = ? AND reference_id = ?", repository.PaymentProviderMidtrans, t.OrderID).First(&p).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("payment untuk order_id %s: %w", t.OrderID, ErrNotFound)
		}
		if err != nil {
			return err
		}
		paymentID = p.ID
		info := map[string]any{}
		if t.TransactionID != "" && (p.ProviderPaymentID == nil || *p.ProviderPaymentID != t.TransactionID) {
			info["provider_payment_id"] = truncate(t.TransactionID, 100)
		}
		if t.PaymentType != "" {
			info["payment_method"] = truncate(strings.ToUpper(t.PaymentType), 50)
		}
		if ch := t.Channel(); ch != "" {
			info["payment_channel"] = truncate(ch, 50)
		}
		if len(info) > 0 {
			if err := tx.Model(&p).Updates(info).Error; err != nil {
				return err
			}
		}
		switch s := t.Status(); {
		case t.Paid():
			return applyCompleted(tx, &p, t)
		case s == midtrans.StatusExpire:
			return closePending(tx, &p, repository.PaymentExpired)
		case s == midtrans.StatusCancel:
			return closePending(tx, &p, repository.PaymentCancelled)
		case s == midtrans.StatusDeny, s == midtrans.StatusFailure, s == midtrans.StatusCapture && t.Fraud() == midtrans.FraudDeny:
			if p.Status == repository.PaymentPending {
				return markFailed(tx, &p, "ditolak Midtrans: "+s)
			}
		}
		return nil // pending/authorize/challenge/refund: belum ada perubahan
	})
	return paymentID, err
}

func applyCompleted(tx *gorm.DB, p *repository.Payment, t *midtrans.Transaction) error {
	if p.Status == repository.PaymentPaid {
		return nil // idempoten
	}
	amount, err := t.AmountRupiah()
	if err != nil || amount != p.Amount || (t.Currency != "" && t.Currency != "IDR") {
		return markFailed(tx, p, fmt.Sprintf("nominal transaksi %s %s ≠ %d IDR", t.GrossAmount, t.Currency, p.Amount))
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
		return markFailed(tx, p, "pembayaran ganda untuk order yang sudah lunas — refund manual lewat Dashboard Midtrans")
	}
	now := time.Now()
	if err := tx.Model(p).Updates(map[string]any{"status": repository.PaymentPaid, "paid_at": now, "failure_reason": nil}).Error; err != nil {
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
		// Jejaknya tetap terlihat di order: status PAID tapi cancelled_at/cancel_reason terisi.
		log.Printf("pembayaran telat: order %s %s → PAID (payment %d)", order.OrderNo, prev, p.ID)
		return nil
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

// CancelOrderSessions mematikan transaksi Midtrans yang masih pending untuk order
// (mis. setelah pembeli membatalkan order), supaya tidak bisa dibayar. Best effort.
func CancelOrderSessions(ctx context.Context, orderID uint) {
	if !configured() {
		return
	}
	var pending []repository.Payment
	if err := config.DB.Where("order_id = ? AND status = ? AND provider = ? AND provider_session_id IS NOT NULL",
		orderID, repository.PaymentPending, repository.PaymentProviderMidtrans).Find(&pending).Error; err != nil {
		log.Printf("batalkan pembayaran order %d: %v", orderID, err)
		return
	}
	for _, p := range pending {
		if err := closeAtGateway(ctx, p, repository.PaymentCancelled); err != nil {
			log.Printf("batalkan payment %d: %v", p.ID, err)
		}
	}
}

// closeAtGateway: expire transaksi di Midtrans lalu tutup payment sebagai closeAs.
//   - Kalau ternyata sudah lunas (412), status sebenarnya diterapkan (bukan dibatalkan).
//   - Kalau pembeli belum memilih metode bayar, tidak ada transaksi yang bisa
//     dimatikan: payment ditutup di sisi kita dan halaman Snap habis sendiri. Kalau
//     tetap dibayar, notifikasinya diproses sebagai pembayaran telat.
//
// Error jaringan/5xx dikembalikan supaya dicoba lagi.
func closeAtGateway(ctx context.Context, p repository.Payment, closeAs string) error {
	id := gatewayID(p)
	t, err := gateway.Expire(ctx, id)
	var apiErr *midtrans.APIError
	if errors.As(err, &apiErr) && apiErr.Rejected() {
		t, err = gateway.GetStatus(ctx, id) // mis. 412: sudah tidak pending → ambil status sebenarnya
	}
	switch {
	case errors.Is(err, midtrans.ErrTransactionNotFound):
		return closeLocal(p.ID, closeAs)
	case err != nil:
		return err
	case t.OrderID != "" && t.OrderID != p.ReferenceID:
		return fmt.Errorf("%w: order_id %q ≠ %q", ErrValidation, t.OrderID, p.ReferenceID)
	case t.Status() == midtrans.StatusExpire:
		return closeLocal(p.ID, closeAs)
	}
	_, err = applyTransaction(t)
	return err
}

// closeLocal menutup payment yang masih PENDING (tidak menimpa PAID dari notifikasi
// yang datang bersamaan).
func closeLocal(paymentID uint, status string) error {
	return config.DB.Model(&repository.Payment{}).Where("id = ? AND status = ?", paymentID, repository.PaymentPending).
		Update("status", status).Error
}

// ReconcileAndExpire dipanggil berkala dari main.go:
//  1. payment PENDING yang > 10 menit dicek ke Midtrans (notifikasi terlewat);
//  2. order yang lewat batas bayar: transaksinya di-expire dulu, baru order EXPIRED.
//     Kalau expire gagal karena jaringan, order dilewati sampai putaran berikutnya
//     (supaya tidak ada transaksi pending yang masih hidup untuk order EXPIRED).
func ReconcileAndExpire(ctx context.Context, now time.Time) (expired int, err error) {
	if !configured() {
		return orderservices.ExpireOverdueOrders(now)
	}
	var stale []repository.Payment
	if err := config.DB.Where("provider = ? AND status = ? AND provider_session_id IS NOT NULL AND created_at < ?",
		repository.PaymentProviderMidtrans, repository.PaymentPending, now.Add(-reconcileAfter)).Order("id").Find(&stale).Error; err != nil {
		return 0, err
	}
	for _, p := range stale {
		if _, _, err := syncTransaction(ctx, gatewayID(p), p.ReferenceID); err != nil {
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
			id, repository.PaymentPending, repository.PaymentProviderMidtrans).Find(&pending).Error; err != nil {
			return expired, err
		}
		ok := true
		for _, p := range pending {
			if err := closeAtGateway(ctx, p, repository.PaymentExpired); err != nil {
				log.Printf("expire order %d: payment %d gagal di-expire, dicoba lagi nanti: %v", id, p.ID, err)
				ok = false
			}
		}
		if !ok {
			continue
		}
		// percobaan yang transaksinya tidak pernah jadi (create gagal di tengah jalan),
		// atau sisa gateway lama
		if err := config.DB.Model(&repository.Payment{}).
			Where("order_id = ? AND status = ? AND (provider_session_id IS NULL OR provider <> ?)", id, repository.PaymentPending, repository.PaymentProviderMidtrans).
			Updates(map[string]any{"status": repository.PaymentCancelled, "failure_reason": "order kedaluwarsa"}).Error; err != nil {
			return expired, err
		}
		done, err := orderservices.ExpireOrder(id) // false kalau ternyata sudah PAID lewat status di atas
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
		CheckoutURL: p.CheckoutURL, SnapToken: p.ProviderSessionID, Amount: p.Amount, Currency: p.Currency,
		ExpiresAt: p.ExpiresAt, PaidAt: p.PaidAt, CreatedAt: p.CreatedAt}
}

// truncate memotong s menjadi maksimal n karakter (tidak memotong di tengah huruf UTF-8).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}
