package services

import (
	"backend/src/repository"
	"net/url"
	"testing"
	"time"
)

func TestBuildSessionRequest(t *testing.T) {
	Configure(nil, Settings{SuccessURL: "https://shop.test/bayar/sukses?x=1", CancelURL: "https://shop.test/bayar/batal"})
	exp := time.Now().Add(24 * time.Hour)
	order := &repository.Order{OrderNo: "SP2611100ABC", Total: 190000}
	pay := &repository.Payment{ReferenceID: "ref-1", Amount: 190000, ExpiresAt: &exp}
	vn := "Dolly Girl, 30 ml, Murni"
	items := []repository.OrderItem{{ProductName: "Annasui", VariantName: &vn, VariantCode: "annasui-dolly-girl-30-murni", UnitPrice: 95000, Quantity: 2, Subtotal: 190000}}
	req := buildSessionRequest(order, pay, items)
	if req.Amount != 190000 || req.Currency != "IDR" || req.Country != "ID" || req.SessionType != "PAY" || req.Mode != "PAYMENT_LINK" || req.ReferenceID != "ref-1" {
		t.Fatalf("field wajib salah: %+v", req)
	}
	if len(req.Items) != 1 || req.Items[0].Name != "Annasui - Dolly Girl, 30 ml, Murni" || req.Items[0].NetUnitAmount != 95000 || req.Items[0].Quantity != 2 {
		t.Fatalf("items salah: %+v", req.Items)
	}
	u, _ := url.Parse(req.SuccessReturnURL)
	if u.Query().Get("order") != "SP2611100ABC" || u.Query().Get("x") != "1" || u.Scheme != "https" {
		t.Fatalf("success url salah: %s", req.SuccessReturnURL)
	}
	// Jumlah item ≠ total (mis. nanti ada diskon) → items tidak dikirim.
	pay.Amount, order.Total = 180000, 180000
	if req := buildSessionRequest(order, pay, items); req.Items != nil {
		t.Fatal("items harus dikosongkan kalau jumlahnya beda dengan amount")
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
}
