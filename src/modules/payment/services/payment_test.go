package services

import (
	"backend/src/modules/payment/doku"
	"backend/src/repository"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestInvoiceNumber(t *testing.T) {
	// Nomor pesanan asli: "SP" + yymmdd + 8 karakter acak.
	inv := invoiceNumber("SP261116A1B2C3D4", 1)
	if inv != "SP261116A1B2C3D4P01" {
		t.Fatalf("invoice = %q", inv)
	}
	// Batas DOKU: maks 30 karakter kalau kartu kredit aktif, tanpa simbol untuk KKI.
	if !regexp.MustCompile(`^[A-Za-z0-9]{1,30}$`).MatchString(invoiceNumber("SP261116A1B2C3D4", 123)) {
		t.Fatalf("invoice percobaan ke-123 tidak aman: %q", invoiceNumber("SP261116A1B2C3D4", 123))
	}
}

func TestBuildCheckoutRequest(t *testing.T) {
	Configure(nil, Settings{SuccessURL: "https://shop.test/bayar/sukses?x=1"})
	now := time.Date(2026, 11, 16, 3, 26, 30, 0, time.UTC)
	exp := now.Add(90 * time.Minute)
	order := &repository.Order{OrderNo: "SP261116A1B2C3D4", Total: 190000}
	pay := &repository.Payment{ReferenceID: "SP261116A1B2C3D4P01", Amount: 190000, ExpiresAt: &exp}
	vn := "Lavender, 30 ml, Murni"
	items := []repository.OrderItem{{ProductName: "Parfum A", VariantName: &vn, VariantCode: "parfum-a-lavender-30-murni", UnitPrice: 95000, Quantity: 2, Subtotal: 190000}}
	cust := &doku.Customer{Name: "Budi", Email: "budi@test.local"}
	req := buildCheckoutRequest(order, pay, items, cust, now)
	o := req.Order
	if o.InvoiceNumber != "SP261116A1B2C3D4P01" || o.Amount != 190000 || o.Currency != "IDR" || !o.AutoRedirect || req.Customer != cust {
		t.Fatalf("field wajib salah: %+v", req)
	}
	if len(o.LineItems) != 1 || o.LineItems[0].Name != "Parfum A - Lavender, 30 ml, Murni" || o.LineItems[0].Price != 95000 ||
		o.LineItems[0].Quantity != 2 || len(o.LineItems[0].ID) > 64 {
		t.Fatalf("items salah: %+v", o.LineItems)
	}
	// payment_due_date = menit sampai batas bayar, dihitung sejak halaman dibuat.
	if req.Payment.PaymentDueDate != 90 {
		t.Fatalf("payment_due_date = %d", req.Payment.PaymentDueDate)
	}
	for _, cb := range []string{o.CallbackURL, o.CallbackURLResult} {
		u, _ := url.Parse(cb)
		if u.Query().Get("order") != "SP261116A1B2C3D4" || u.Query().Get("x") != "1" || u.Scheme != "https" {
			t.Fatalf("callback url salah: %s", cb)
		}
	}
	// Jumlah item ≠ total (mis. nanti ada diskon) → items tidak dikirim.
	pay.Amount, order.Total = 180000, 180000
	if req := buildCheckoutRequest(order, pay, items, nil, now); req.Order.LineItems != nil || req.Customer != nil {
		t.Fatal("items harus dikosongkan kalau jumlahnya beda dengan amount")
	}
	// Nama item maks 255 karakter, tidak memotong huruf UTF-8.
	long := strings.Repeat("é", 300)
	items[0].ProductName, items[0].VariantName = long, nil
	pay.Amount = 190000
	if req := buildCheckoutRequest(order, pay, items, nil, now); len([]rune(req.Order.LineItems[0].Name)) != 255 ||
		!strings.HasPrefix(long, req.Order.LineItems[0].Name) {
		t.Fatalf("nama dipotong salah: %q", req.Order.LineItems[0].Name)
	}
}

func TestSessionExpiry(t *testing.T) {
	Configure(nil, Settings{MaxSessionTTL: 2 * time.Hour})
	now := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	if got := sessionExpiry(now, now.Add(24*time.Hour)); !got.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("harus dibatasi MaxSessionTTL: %v", got)
	}
	if got := sessionExpiry(now, now.Add(time.Hour)); !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("batas order lebih awal harus dipakai: %v", got)
	}
	// Dibulatkan ke bawah ke menit penuh dari now (payment_due_date dalam menit).
	if got := sessionExpiry(now, now.Add(time.Hour+59*time.Second)); !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("harus dibulatkan ke menit: %v", got)
	}
}

func TestConfigureFromEnv(t *testing.T) {
	set := func(id, key, prod, success, api string) error {
		t.Setenv("DOKU_CLIENT_ID", id)
		t.Setenv("DOKU_SECRET_KEY", key)
		t.Setenv("DOKU_IS_PRODUCTION", prod)
		t.Setenv("DOKU_API_URL", api)
		t.Setenv("CHECKOUT_SUCCESS_URL", success)
		t.Setenv("PAYMENT_SESSION_MAX_TTL", "")
		return ConfigureFromEnv()
	}
	ok := []struct{ id, key, prod, success, api string }{
		{"BRN-1", "SK-1", "", "https://shop.test/bayar/sukses", ""},
		{"BRN-1", "SK-1", "false", "http://localhost:3000/bayar/sukses", ""},
		{"BRN-1", "SK-1", "true", "https://stationparfume.com/bayar/sukses", ""},
		{"BRN-1", "SK-1", "", "https://shop.test/x", "http://127.0.0.1:9999"},
	}
	for _, c := range ok {
		if err := set(c.id, c.key, c.prod, c.success, c.api); err != nil || !configured() || settings.ClientID != c.id || settings.SecretKey != c.key {
			t.Errorf("%+v harus valid: %v", c, err)
		}
	}
	bad := []struct{ id, key, prod, success, api string }{
		{"", "SK-1", "", "https://shop.test/x", ""},             // client id kosong
		{"BRN-1", "", "", "https://shop.test/x", ""},            // secret kosong
		{"BRN-1", "SK-1", "", "", ""},                           // URL kosong
		{"BRN-1", "SK-1", "", "http://shop.test/x", ""},         // http selain localhost
		{"BRN-1", "SK-1", "ya", "https://shop.test/x", ""},      // bool tidak valid
		{"BRN-1", "SK-1", "", "https://shop.test/x", "ftp://x"}, // API URL aneh
	}
	for _, c := range bad {
		if err := set(c.id, c.key, c.prod, c.success, c.api); !errors.Is(err, ErrNotConfigured) || configured() {
			t.Errorf("%+v harus ditolak: %v", c, err)
		}
	}
	set("BRN-1", "SK-1", "", "https://shop.test/x", "")
	t.Setenv("PAYMENT_SESSION_MAX_TTL", "5m")
	if err := ConfigureFromEnv(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("TTL < 15m harus ditolak: %v", err)
	}
	Configure(nil, Settings{})
}
