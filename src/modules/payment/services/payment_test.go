package services

import (
	"backend/src/repository"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBuildSnapRequest(t *testing.T) {
	Configure(nil, Settings{SuccessURL: "https://shop.test/bayar/sukses?x=1"})
	now := time.Date(2026, 11, 12, 3, 26, 30, 0, time.UTC)
	exp := now.Add(90 * time.Minute)
	order := &repository.Order{OrderNo: "SP2611100ABC", Total: 190000}
	pay := &repository.Payment{ReferenceID: "ref-1", Amount: 190000, ExpiresAt: &exp}
	vn := "Dolly Girl, 30 ml, Murni"
	items := []repository.OrderItem{{ProductName: "Annasui", VariantName: &vn, VariantCode: "annasui-dolly-girl-30-murni", UnitPrice: 95000, Quantity: 2, Subtotal: 190000}}
	req := buildSnapRequest(order, pay, items, now)
	if req.TransactionDetails.OrderID != "ref-1" || req.TransactionDetails.GrossAmount != 190000 || req.CustomField1 != "SP2611100ABC" {
		t.Fatalf("field wajib salah: %+v", req)
	}
	if len(req.ItemDetails) != 1 || req.ItemDetails[0].Name != "Annasui - Dolly Girl, 30 ml, Murni" || req.ItemDetails[0].Price != 95000 ||
		req.ItemDetails[0].Quantity != 2 || len(req.ItemDetails[0].ID) > 50 {
		t.Fatalf("items salah: %+v", req.ItemDetails)
	}
	// start_time dalam WIB, durasi menit sampai batas bayar.
	if req.Expiry == nil || req.Expiry.StartTime != "2026-11-12 10:26:30 +0700" || req.Expiry.Unit != "minute" || req.Expiry.Duration != 90 ||
		req.PageExpiry == nil || req.PageExpiry.Duration != 90 {
		t.Fatalf("expiry salah: %+v %+v", req.Expiry, req.PageExpiry)
	}
	u, _ := url.Parse(req.Callbacks.Finish)
	if u.Query().Get("order") != "SP2611100ABC" || u.Query().Get("x") != "1" || u.Scheme != "https" {
		t.Fatalf("finish url salah: %s", req.Callbacks.Finish)
	}
	// Jumlah item ≠ total (mis. nanti ada diskon) → items tidak dikirim (Midtrans menolak).
	pay.Amount, order.Total = 180000, 180000
	if req := buildSnapRequest(order, pay, items, now); req.ItemDetails != nil {
		t.Fatal("items harus dikosongkan kalau jumlahnya beda dengan amount")
	}
	// Nama item maks 50 karakter, tidak memotong huruf UTF-8.
	long := strings.Repeat("é", 60)
	items[0].ProductName, items[0].VariantName = long, nil
	pay.Amount = 190000
	if req := buildSnapRequest(order, pay, items, now); len([]rune(req.ItemDetails[0].Name)) != 50 || !strings.HasPrefix(long, req.ItemDetails[0].Name) {
		t.Fatalf("nama dipotong salah: %q", req.ItemDetails[0].Name)
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
	// Dibulatkan ke bawah ke menit penuh dari now (expiry Midtrans dalam menit).
	if got := sessionExpiry(now, now.Add(time.Hour+59*time.Second)); !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("harus dibulatkan ke menit: %v", got)
	}
}

func TestConfigureFromEnv(t *testing.T) {
	set := func(key, prod, success string) error {
		t.Setenv("MIDTRANS_SERVER_KEY", key)
		t.Setenv("MIDTRANS_IS_PRODUCTION", prod)
		t.Setenv("CHECKOUT_SUCCESS_URL", success)
		t.Setenv("PAYMENT_SESSION_MAX_TTL", "")
		return ConfigureFromEnv()
	}
	ok := []struct{ key, prod, success string }{
		{"SB-Mid-server-abc", "", "https://shop.test/bayar/sukses"},
		{"SB-Mid-server-abc", "false", "http://localhost:3000/bayar/sukses"},
		{"Mid-server-abc", "true", "https://stationparfume.com/bayar/sukses"},
	}
	for _, c := range ok {
		if err := set(c.key, c.prod, c.success); err != nil || !configured() || settings.ServerKey != c.key {
			t.Errorf("%+v harus valid: %v", c, err)
		}
	}
	bad := []struct{ key, prod, success string }{
		{"", "", "https://shop.test/x"},                      // key kosong
		{"SB-Mid-server-abc", "", ""},                        // URL kosong
		{"SB-Mid-server-abc", "", "http://shop.test/x"},      // http selain localhost
		{"SB-Mid-server-abc", "true", "https://shop.test/x"}, // key sandbox di production
		{"Mid-server-abc", "", "https://shop.test/x"},        // key production di sandbox
		{"SB-Mid-server-abc", "ya", "https://shop.test/x"},   // bool tidak valid
	}
	for _, c := range bad {
		if err := set(c.key, c.prod, c.success); !errors.Is(err, ErrNotConfigured) || configured() {
			t.Errorf("%+v harus ditolak: %v", c, err)
		}
	}
	t.Setenv("PAYMENT_SESSION_MAX_TTL", "5m")
	if err := ConfigureFromEnv(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("TTL < 15m harus ditolak: %v", err)
	}
	Configure(nil, Settings{})
}
