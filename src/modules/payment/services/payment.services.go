// Package services: pembayaran lewat DOKU Checkout (hosted payment page).
//
// Alur: POST /orders (checkout) → POST /orders/:id/pay → redirect ke checkout_url
// (halaman DOKU Checkout) → notifikasi DOKU → status dicek ulang lewat Check Status →
// order PAID. Isi notifikasi TIDAK pernah dipercaya; status PAID hanya dari GetStatus.
//
// Satu baris payments = satu halaman Checkout: payments.reference_id dikirim sebagai
// order.invoice_number (nomor pesanan + "P" + nomor percobaan), provider_session_id =
// token_id, provider_payment_request_id = Request-Id create (dibutuhkan API cancel),
// checkout_url = payment.url.
package services

import (
	"backend/src/config"
	orderservices "backend/src/modules/order/services"
	"backend/src/modules/payment/doku"
	"backend/src/modules/payment/models"
	"backend/src/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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
	// ErrRetryLater: notifikasi belum bisa diproses (status di DOKU belum sesuai
	// notifikasi) → dijawab 503 supaya DOKU mengirim ulang.
	ErrRetryLater = errors.New("retry later")
)

// Gateway = bagian client DOKU yang dipakai service (diganti server palsu saat test).
type Gateway interface {
	CreatePayment(ctx context.Context, req doku.PaymentRequest) (*doku.PaymentResponse, error)
	GetStatus(ctx context.Context, invoice string) (*doku.Status, error)
	Cancel(ctx context.Context, invoice, originalRequestID, note string) error
}

type Settings struct {
	ClientID      string        // Client-Id notifikasi harus sama dengan ini
	SecretKey     string        // untuk memverifikasi Signature notifikasi
	SuccessURL    string        // halaman FE "memverifikasi pembayaran" (callback_url)
	MaxSessionTTL time.Duration // batas atas umur halaman Checkout (default 24 jam)
	// RecheckDelay: jeda cek status ulang saat notifikasi SUCCESS tapi DOKU masih
	// PENDING (default 2 detik, 2× sebelum dijawab 503).
	RecheckDelay time.Duration
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
	if s.RecheckDelay <= 0 {
		s.RecheckDelay = 2 * time.Second
	}
	gateway, settings = g, s
}

// ConfigureFromEnv membaca DOKU_CLIENT_ID, DOKU_SECRET_KEY, CHECKOUT_SUCCESS_URL
// (wajib), DOKU_IS_PRODUCTION (opsional, default false = sandbox), DOKU_API_URL
// (opsional, override URL), PAYMENT_SESSION_MAX_TTL (opsional, mis. "24h"). Kalau
// ada yang kosong/salah, pembayaran dimatikan (endpoint bayar & webhook menjawab
// 503) dan error dikembalikan untuk di-log.
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
	clientID, secret, success := get("DOKU_CLIENT_ID"), get("DOKU_SECRET_KEY"), get("CHECKOUT_SUCCESS_URL")
	if len(missing) > 0 {
		return fail("env kosong: %s", strings.Join(missing, ", "))
	}
	if !validReturnURL(success) {
		return fail("CHECKOUT_SUCCESS_URL harus URL HTTPS (http hanya untuk localhost)")
	}
	production := false
	if v := strings.TrimSpace(os.Getenv("DOKU_IS_PRODUCTION")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fail("DOKU_IS_PRODUCTION %q tidak valid (true/false)", v)
		}
		production = b
	}
	apiURL := strings.TrimSpace(os.Getenv("DOKU_API_URL"))
	if apiURL != "" && !validReturnURL(apiURL) {
		return fail("DOKU_API_URL %q harus URL HTTPS", apiURL)
	}
	ttl := 24 * time.Hour
	if v := os.Getenv("PAYMENT_SESSION_MAX_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 15*time.Minute {
			return fail("PAYMENT_SESSION_MAX_TTL %q tidak valid (mis. 24h, minimal 15m)", v)
		}
		ttl = d
	}
	Configure(doku.New(clientID, secret, production, apiURL),
		Settings{ClientID: clientID, SecretKey: secret, SuccessURL: success, MaxSessionTTL: ttl})
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
	// dibuat oleh request lain (jangan buat halaman Checkout kedua).
	pendingCreateGrace = 30 * time.Second
	// Sisa waktu bayar minimal supaya halaman Checkout baru masih masuk akal.
	minSessionTTL = 11 * time.Minute
	// Rekonsiliasi: payment PENDING lebih tua dari ini dicek ke DOKU (notifikasi terlewat).
	reconcileAfter = 10 * time.Minute
	// Cek status ulang sebelum notifikasi SUCCESS dijawab 503.
	recheckAttempts = 2
)

// PayOrder membuat (atau memakai ulang) halaman DOKU Checkout untuk order milik
// user. Idempoten: selama ada payment PENDING yang belum kedaluwarsa, checkout_url
// yang sama dikembalikan.
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
		case err == nil && existing.Provider == repository.PaymentProviderDoku && existing.CheckoutURL != nil &&
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
		// Nomor percobaan dihitung di bawah lock order (termasuk baris soft-delete,
		// karena reference_id unik untuk semua baris).
		var attempts int64
		if err := tx.Unscoped().Model(&repository.Payment{}).Where("order_id = ?", order.ID).Count(&attempts).Error; err != nil {
			return err
		}
		expires := sessionExpiry(now, *order.ExpiresAt)
		pay = repository.Payment{OrderID: order.ID, Provider: repository.PaymentProviderDoku,
			ReferenceID: invoiceNumber(order.OrderNo, attempts+1), Amount: order.Total, Currency: "IDR",
			Status: repository.PaymentPending, ExpiresAt: &expires}
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
	var buyer repository.User
	var customer *doku.Customer
	if err := config.DB.Select("id", "name", "email").First(&buyer, order.UserID).Error; err == nil {
		customer = &doku.Customer{Name: truncate(buyer.Name, 255), Email: truncate(buyer.Email, 128)}
	}
	res, gwErr := gateway.CreatePayment(ctx, buildCheckoutRequest(order, &pay, items, customer, now))
	if gwErr != nil {
		reason := truncate(gwErr.Error(), 500)
		config.DB.Model(&pay).Updates(map[string]any{"status": repository.PaymentFailed, "failure_reason": reason})
		log.Printf("doku create checkout order %s: %v", order.OrderNo, gwErr)
		return nil, fmt.Errorf("%w: gagal membuat transaksi pembayaran, coba lagi", ErrGateway)
	}
	upd := map[string]any{"provider_session_id": truncate(res.TokenID, 100), "provider_payment_request_id": truncate(res.RequestID, 100),
		"checkout_url": res.URL, "provider_response": datatypes.JSON(res.Raw)}
	if err := config.DB.Model(&pay).Updates(upd).Error; err != nil {
		return nil, err
	}
	if err := config.DB.First(&pay, pay.ID).Error; err != nil {
		return nil, err
	}
	return toResponse(pay), nil
}

// invoiceNumber: invoice DOKU = nomor pesanan + "P" + nomor percobaan 2 digit, mis.
// SP261116A1B2C3D4P01. Alfanumerik dan ≤30 karakter (batas kalau kartu kredit aktif;
// KKI tidak menerima simbol), dan mudah dicocokkan pemilik di dashboard DOKU.
func invoiceNumber(orderNo string, attempt int64) string {
	return fmt.Sprintf("%sP%02d", orderNo, attempt)
}

// sessionExpiry: batas halaman Checkout = batas bayar order, dibatasi MaxSessionTTL,
// dibulatkan ke bawah ke menit penuh dari `now` (payment_due_date DOKU dalam menit).
func sessionExpiry(now, orderExpiry time.Time) time.Time {
	exp := orderExpiry
	if limit := now.Add(settings.MaxSessionTTL); exp.After(limit) {
		exp = limit
	}
	return now.Add(exp.Sub(now).Truncate(time.Minute))
}

func buildCheckoutRequest(order *repository.Order, pay *repository.Payment, items []repository.OrderItem, customer *doku.Customer, now time.Time) doku.PaymentRequest {
	finish := withOrderParam(settings.SuccessURL, order.OrderNo)
	req := doku.PaymentRequest{
		Order: doku.OrderRequest{Amount: pay.Amount, InvoiceNumber: pay.ReferenceID, Currency: "IDR",
			CallbackURL: finish, CallbackURLResult: finish, AutoRedirect: true},
		Customer: customer,
	}
	if pay.ExpiresAt != nil {
		if minutes := int(pay.ExpiresAt.Sub(now) / time.Minute); minutes > 0 {
			req.Payment.PaymentDueDate = minutes // dihitung sejak halaman dibuat
		}
	}
	// Rincian item hanya dikirim kalau jumlahnya persis sama dengan total (DOKU
	// mensyaratkan price×quantity = amount; ongkir & diskon sekarang 0, jadi biasanya sama).
	var sum int64
	list := make([]doku.LineItem, 0, len(items))
	for _, it := range items {
		name := it.ProductName
		if it.VariantName != nil && *it.VariantName != "" {
			name += " - " + *it.VariantName
		}
		list = append(list, doku.LineItem{ID: truncate(it.VariantCode, 64), Name: truncate(name, 255), Price: it.UnitPrice,
			Quantity: it.Quantity, Category: "Parfum"})
		sum += it.Subtotal
	}
	if sum == pay.Amount {
		req.Order.LineItems = list
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

// HandleWebhook memproses notifikasi DOKU. Signature header diverifikasi
// (constant-time, Request-Target = path), notifikasi mentah disimpan (dedup), lalu
// status dicek ulang lewat GetStatus. Error yang dikembalikan → handler menjawab
// non-2xx supaya DOKU mengirim ulang.
func HandleWebhook(ctx context.Context, path string, header http.Header, body []byte) error {
	if !configured() || settings.SecretKey == "" {
		return ErrNotConfigured
	}
	if err := doku.VerifyNotification(header, path, body, settings.ClientID, settings.SecretKey); err != nil {
		return ErrUnauthorized
	}
	var n doku.Status
	if err := json.Unmarshal(body, &n); err != nil || n.Order.InvoiceNumber == "" {
		return fmt.Errorf("%w: body notifikasi tidak dikenali", ErrValidation)
	}
	status := n.TxStatus()
	if status == "" && n.OrderStatus() == doku.OrderExpired {
		status = doku.StatusExpired // notifikasi kedaluwarsa halaman Checkout
	}
	if status == "" {
		return fmt.Errorf("%w: notifikasi tanpa transaction.status", ErrValidation)
	}
	invoice := n.Order.InvoiceNumber
	row := repository.PaymentEvent{Provider: repository.PaymentProviderDoku, EventType: truncate(status, 100),
		ProviderPaymentID: truncate(invoice, 100), Payload: datatypes.JSON(body)}
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
	known, err := knownInvoice(invoice)
	if err != nil {
		return err
	}
	if !known {
		// Bukan transaksi toko ini (mis. notifikasi uji dari dashboard): dijawab 200
		// supaya DOKU tidak mengirim ulang, catatannya tetap disimpan.
		return config.DB.Model(&row).Updates(map[string]any{"processed_at": time.Now(),
			"process_error": "invoice tidak dikenal (bukan transaksi toko ini / notifikasi uji)"}).Error
	}
	if status != doku.StatusSuccess && status != doku.StatusExpired {
		// PENDING/FAILED (pembeli masih bisa coba lagi)/REFUNDED/...: disimpan saja
		return config.DB.Model(&row).Update("processed_at", time.Now()).Error
	}
	paymentID, cur, err := syncTransaction(ctx, invoice)
	for i := 0; i < recheckAttempts && err == nil && status == doku.StatusSuccess && cur != doku.StatusSuccess; i++ {
		// DOKU menyarankan cek status ±1 menit setelah bayar: status bisa tertinggal.
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-time.After(settings.RecheckDelay):
			paymentID, cur, err = syncTransaction(ctx, invoice)
		}
	}
	if err == nil && status == doku.StatusSuccess && cur != doku.StatusSuccess {
		// Status di DOKU belum lunas → jangan tandai selesai; DOKU mengirim ulang dan
		// rekonsiliasi berkala juga mengecek payment yang punya notifikasi belum terproses.
		err = fmt.Errorf("%w: notifikasi %s tapi status di DOKU %q", ErrRetryLater, status, cur)
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

// knownInvoice: invoice DOKU milik salah satu payment kita.
func knownInvoice(invoice string) (bool, error) {
	var n int64
	err := config.DB.Model(&repository.Payment{}).
		Where("provider = ? AND reference_id = ?", repository.PaymentProviderDoku, invoice).Count(&n).Error
	return n > 0, err
}

// syncTransaction mengambil status dari DOKU (sumber kebenaran) lalu menerapkannya
// ke payment + order. Mengembalikan id payment kita dan transaction.status
// ("" = DOKU belum mengenal invoice ini).
func syncTransaction(ctx context.Context, invoice string) (uint, string, error) {
	if !configured() {
		return 0, "", ErrNotConfigured
	}
	s, err := gateway.GetStatus(ctx, invoice)
	if errors.Is(err, doku.ErrTransactionNotFound) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("%w: status %s: %v", ErrGateway, invoice, err)
	}
	if s.Order.InvoiceNumber != "" && s.Order.InvoiceNumber != invoice {
		return 0, s.TxStatus(), fmt.Errorf("%w: invoice status %q ≠ %q", ErrValidation, s.Order.InvoiceNumber, invoice)
	}
	pid, err := applyStatus(invoice, s)
	return pid, s.TxStatus(), err
}

func applyStatus(invoice string, s *doku.Status) (uint, error) {
	var paymentID uint
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Payment
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("provider = ? AND reference_id = ?", repository.PaymentProviderDoku, invoice).First(&p).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("payment untuk invoice %s: %w", invoice, ErrNotFound)
		}
		if err != nil {
			return err
		}
		paymentID = p.ID
		info := map[string]any{}
		if m := s.Method(); m != "" {
			info["payment_method"] = truncate(m, 50)
		}
		if ch := s.ChannelName(); ch != "" {
			info["payment_channel"] = truncate(ch, 50)
		}
		if len(info) > 0 {
			if err := tx.Model(&p).Updates(info).Error; err != nil {
				return err
			}
		}
		switch {
		case s.Paid():
			return applyCompleted(tx, &p, s)
		case s.Expired():
			return closePending(tx, &p, repository.PaymentExpired)
		case s.TxStatus() == doku.StatusRefunded:
			log.Printf("payment %d (%s): DOKU melaporkan REFUNDED; status order diurus pemilik", p.ID, p.ReferenceID)
		}
		return nil // PENDING/FAILED/TIMEOUT/REDIRECT: belum ada perubahan
	})
	return paymentID, err
}

func applyCompleted(tx *gorm.DB, p *repository.Payment, s *doku.Status) error {
	if p.Status == repository.PaymentPaid {
		return nil // idempoten
	}
	amount, err := s.AmountRupiah()
	if cur := strings.ToUpper(s.Order.Currency); err != nil || amount != p.Amount || (cur != "" && cur != "IDR") {
		return markFailed(tx, p, fmt.Sprintf("nominal transaksi %s %s ≠ %d IDR", s.Order.Amount, s.Order.Currency, p.Amount))
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
		return markFailed(tx, p, "pembayaran ganda untuk order yang sudah lunas — refund manual lewat dashboard DOKU")
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

// CancelOrderSessions menutup halaman Checkout yang masih pending untuk order (mis.
// setelah pembeli membatalkan order), supaya tidak bisa dibayar. Best effort.
func CancelOrderSessions(ctx context.Context, orderID uint) {
	if !configured() {
		return
	}
	var pending []repository.Payment
	if err := config.DB.Where("order_id = ? AND status = ? AND provider = ? AND provider_session_id IS NOT NULL",
		orderID, repository.PaymentPending, repository.PaymentProviderDoku).Find(&pending).Error; err != nil {
		log.Printf("batalkan pembayaran order %d: %v", orderID, err)
		return
	}
	for _, p := range pending {
		if err := closeAtGateway(ctx, p, repository.PaymentCancelled); err != nil {
			log.Printf("batalkan payment %d: %v", p.ID, err)
		}
	}
}

// closeAtGateway: tutup halaman Checkout di DOKU lalu tutup payment sebagai closeAs.
//   - Sudah lunas di DOKU → status sebenarnya diterapkan (bukan dibatalkan).
//   - Masih PENDING → API cancel. DOKU hanya bisa membatalkan VA & QRIS; kalau
//     ditolak (e-wallet/kartu, atau pembeli belum memilih channel), payment ditutup di
//     sisi kita dan link DOKU habis sendiri pada payment_due_date (≤ batas bayar
//     order). Kalau tetap dibayar, notifikasinya diproses sebagai pembayaran telat.
//
// Error jaringan/5xx dikembalikan supaya dicoba lagi.
func closeAtGateway(ctx context.Context, p repository.Payment, closeAs string) error {
	s, err := gateway.GetStatus(ctx, p.ReferenceID)
	switch {
	case errors.Is(err, doku.ErrTransactionNotFound):
		return closeLocal(p.ID, closeAs)
	case err != nil:
		return err
	case s.Paid():
		_, err = applyStatus(p.ReferenceID, s)
		return err
	case s.Expired():
		return closeLocal(p.ID, closeAs)
	}
	reqID := ""
	if p.ProviderPaymentRequestID != nil {
		reqID = *p.ProviderPaymentRequestID
	}
	note := "pesanan dibatalkan"
	if closeAs == repository.PaymentExpired {
		note = "batas waktu pembayaran pesanan habis"
	}
	if err := gateway.Cancel(ctx, p.ReferenceID, reqID, note); err != nil {
		var apiErr *doku.APIError
		if !errors.As(err, &apiErr) || !apiErr.Rejected() {
			return err
		}
		// Ditolak: cek lagi, siapa tahu baru saja dibayar.
		s2, err2 := gateway.GetStatus(ctx, p.ReferenceID)
		switch {
		case err2 == nil && s2.Paid():
			_, err = applyStatus(p.ReferenceID, s2)
			return err
		case err2 != nil && !errors.Is(err2, doku.ErrTransactionNotFound):
			return err2
		}
		log.Printf("payment %d (%s): DOKU menolak cancel (%v); ditutup di sisi toko, link habis sendiri pada batas waktunya",
			p.ID, p.ReferenceID, apiErr)
	}
	return closeLocal(p.ID, closeAs)
}

// closeLocal menutup payment yang masih PENDING (tidak menimpa PAID dari notifikasi
// yang datang bersamaan).
func closeLocal(paymentID uint, status string) error {
	return config.DB.Model(&repository.Payment{}).Where("id = ? AND status = ?", paymentID, repository.PaymentPending).
		Update("status", status).Error
}

// ReconcileAndExpire dipanggil berkala dari main.go:
//  1. payment PENDING yang > 10 menit, atau yang punya notifikasi belum terproses
//     (mis. SUCCESS saat status DOKU masih tertinggal), dicek ke DOKU;
//  2. order yang lewat batas bayar: halaman Checkout-nya ditutup dulu, baru order
//     EXPIRED. Kalau gagal karena jaringan, order dilewati sampai putaran berikutnya.
func ReconcileAndExpire(ctx context.Context, now time.Time) (expired int, err error) {
	if !configured() {
		return orderservices.ExpireOverdueOrders(now)
	}
	unprocessed := config.DB.Model(&repository.PaymentEvent{}).Select("provider_payment_id").
		Where("provider = ? AND processed_at IS NULL", repository.PaymentProviderDoku)
	var stale []repository.Payment
	if err := config.DB.Where("provider = ? AND status = ? AND provider_session_id IS NOT NULL",
		repository.PaymentProviderDoku, repository.PaymentPending).
		Where("(created_at < ? OR reference_id IN (?))", now.Add(-reconcileAfter), unprocessed).
		Order("id").Find(&stale).Error; err != nil {
		return 0, err
	}
	for _, p := range stale {
		_, cur, err := syncTransaction(ctx, p.ReferenceID)
		if err != nil {
			log.Printf("rekonsiliasi payment %d: %v", p.ID, err)
			continue
		}
		if cur == doku.StatusSuccess {
			// notifikasi yang tadinya dijawab 503 sekarang sudah tertangani
			config.DB.Model(&repository.PaymentEvent{}).
				Where("provider = ? AND provider_payment_id = ? AND processed_at IS NULL", repository.PaymentProviderDoku, p.ReferenceID).
				Updates(map[string]any{"processed_at": time.Now(), "process_error": nil, "payment_id": p.ID})
		}
	}
	ids, err := orderservices.OverdueOrderIDs(now)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		var pending []repository.Payment
		if err := config.DB.Where("order_id = ? AND status = ? AND provider = ? AND provider_session_id IS NOT NULL",
			id, repository.PaymentPending, repository.PaymentProviderDoku).Find(&pending).Error; err != nil {
			return expired, err
		}
		ok := true
		for _, p := range pending {
			if err := closeAtGateway(ctx, p, repository.PaymentExpired); err != nil {
				log.Printf("expire order %d: payment %d gagal ditutup, dicoba lagi nanti: %v", id, p.ID, err)
				ok = false
			}
		}
		if !ok {
			continue
		}
		// percobaan yang halaman Checkout-nya tidak pernah jadi (create gagal di tengah
		// jalan), atau sisa gateway lama
		if err := config.DB.Model(&repository.Payment{}).
			Where("order_id = ? AND status = ? AND (provider_session_id IS NULL OR provider <> ?)", id, repository.PaymentPending, repository.PaymentProviderDoku).
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
		CheckoutURL: p.CheckoutURL, Amount: p.Amount, Currency: p.Currency,
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
