package doku

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func hmacB64(secret, s string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(s))
	return "HMACSHA256=" + base64.StdEncoding.EncodeToString(m.Sum(nil))
}

func TestSign(t *testing.T) {
	body := []byte(`{"order":{"amount":20000,"invoice_number":"INV1"}}`)
	sum := sha256.Sum256(body)
	digest := base64.StdEncoding.EncodeToString(sum[:])
	// Komponen persis seperti contoh docs DOKU: baris dipisah \n, tanpa \n di akhir.
	post := "Client-Id:BRN-1\nRequest-Id:r-1\nRequest-Timestamp:2026-11-16T03:00:00Z\nRequest-Target:/checkout/v1/payment\nDigest:" + digest
	if got := Sign("BRN-1", "r-1", "2026-11-16T03:00:00Z", "/checkout/v1/payment", body, "sk"); got != hmacB64("sk", post) {
		t.Fatalf("POST signature salah: %s", got)
	}
	// GET: baris Digest tidak ada sama sekali.
	get := "Client-Id:BRN-1\nRequest-Id:r-2\nRequest-Timestamp:2026-11-16T03:00:00Z\nRequest-Target:/orders/v1/status/INV1"
	if got := Sign("BRN-1", "r-2", "2026-11-16T03:00:00Z", "/orders/v1/status/INV1", nil, "sk"); got != hmacB64("sk", get) {
		t.Fatalf("GET signature salah: %s", got)
	}
}

// checkSigned memastikan request dari client bertanda tangan benar.
func checkSigned(t *testing.T, r *http.Request, body []byte) {
	t.Helper()
	if r.Header.Get("Client-Id") != "BRN-1" || r.Header.Get("Request-Id") == "" {
		t.Fatalf("header kurang: %v", r.Header)
	}
	if _, err := time.Parse(TimestampLayout, r.Header.Get("Request-Timestamp")); err != nil {
		t.Fatalf("Request-Timestamp bukan UTC ISO8601: %q", r.Header.Get("Request-Timestamp"))
	}
	if r.Method == http.MethodGet {
		body = nil
	}
	want := Sign("BRN-1", r.Header.Get("Request-Id"), r.Header.Get("Request-Timestamp"), r.URL.EscapedPath(), body, "sk")
	if r.Header.Get("Signature") != want {
		t.Fatalf("signature %s %s salah", r.Method, r.URL.Path)
	}
}

func TestCreatePayment(t *testing.T) {
	var gotPath, gotReqID string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		checkSigned(t, r, b)
		gotPath, gotReqID = r.Method+" "+r.URL.Path, r.Header.Get("Request-Id")
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"message":["SUCCESS"],"response":{"order":{"amount":"95000","invoice_number":"SP1P01","session_id":"s1"},
			"payment":{"payment_due_date":90,"token_id":"tok-1","url":"https://sandbox.doku.com/checkout-link-v2/tok-1","expired_date":"20261116120000"}}}`))
	}))
	defer srv.Close()
	c := New("BRN-1", "sk", false, srv.URL)
	p, err := c.CreatePayment(context.Background(), PaymentRequest{
		Order: OrderRequest{Amount: 95000, InvoiceNumber: "SP1P01", CallbackURL: "https://shop.test/sukses?order=SP1", AutoRedirect: true,
			LineItems: []LineItem{{ID: "v", Name: "Parfum", Price: 95000, Quantity: 1}}},
		Payment:  PaymentOptions{PaymentDueDate: 90},
		Customer: &Customer{Name: "Budi", Email: "budi@test.local"},
	})
	if err != nil {
		t.Fatal(err)
	}
	order, _ := got["order"].(map[string]any)
	pay, _ := got["payment"].(map[string]any)
	if gotPath != "POST /checkout/v1/payment" || order["invoice_number"] != "SP1P01" || order["amount"] != float64(95000) ||
		order["auto_redirect"] != true || pay["payment_due_date"] != float64(90) || len(order["line_items"].([]any)) != 1 {
		t.Fatalf("request salah: %s %v", gotPath, got)
	}
	if p.URL == "" || p.TokenID != "tok-1" || p.ExpiredDate != "20261116120000" || p.RequestID != gotReqID || len(p.Raw) == 0 {
		t.Fatalf("respons salah: %+v", p)
	}
}

func TestCreatePaymentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error_messages":["order.invoice_number must be filled","order.amount must greater than 0"]}`))
	}))
	defer srv.Close()
	_, err := New("BRN-1", "sk", false, srv.URL).CreatePayment(context.Background(), PaymentRequest{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.HTTPStatus != 400 || len(apiErr.Messages) != 2 || !apiErr.Rejected() {
		t.Fatalf("error salah: %v", err)
	}
}

func TestGetStatusAndCancel(t *testing.T) {
	reply, httpStatus := "", 200
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		checkSigned(t, r, b)
		gotPath, gotBody = r.Method+" "+r.URL.Path, nil
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(httpStatus)
		_, _ = w.Write([]byte(reply))
	}))
	defer srv.Close()
	c := New("BRN-1", "sk", false, srv.URL)
	ctx := context.Background()

	reply = `{"order":{"invoice_number":"SP1P01","amount":95000},"transaction":{"status":"SUCCESS","date":"2026-11-16T03:00:00Z"},
		"service":{"id":"VIRTUAL_ACCOUNT"},"acquirer":{"id":"BRI"},"channel":{"id":"VIRTUAL_ACCOUNT_BRI"}}`
	s, err := c.GetStatus(ctx, "SP1P01")
	if err != nil || gotPath != "GET /orders/v1/status/SP1P01" || !s.Paid() || s.Expired() || s.Method() != "VIRTUAL_ACCOUNT" ||
		s.ChannelName() != "VIRTUAL_ACCOUNT_BRI" || len(s.Raw) == 0 {
		t.Fatalf("status: %v %s %+v", err, gotPath, s)
	}
	if a, err := s.AmountRupiah(); err != nil || a != 95000 {
		t.Fatalf("amount: %d %v", a, err)
	}

	// Belum memilih channel: PENDING + ORDER_GENERATED, channel kosong; amount berupa string.
	reply = `{"order":{"invoice_number":"SP1P01","amount":"95000.00","status":"ORDER_GENERATED"},"transaction":{"status":"PENDING"},
		"service":{"id":""},"acquirer":{"id":""},"channel":{"id":""}}`
	if s, err := c.GetStatus(ctx, "SP1P01"); err != nil || s.Paid() || s.Expired() || s.ChannelName() != "" || s.OrderStatus() != OrderGenerated {
		t.Fatalf("pending: %v %+v", err, s)
	} else if a, _ := s.AmountRupiah(); a != 95000 {
		t.Fatalf("amount string: %d", a)
	}

	reply = `{"order":{"invoice_number":"SP1P01","amount":95000,"status":"ORDER_EXPIRED"},"transaction":{"status":"PENDING"}}`
	if s, err := c.GetStatus(ctx, "SP1P01"); err != nil || !s.Expired() {
		t.Fatalf("expired: %v %+v", err, s)
	}

	httpStatus, reply = 404, `{"error":{"message":"Not Found"}}`
	if _, err := c.GetStatus(ctx, "SP1P01"); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("404: %v", err)
	}

	httpStatus, reply = 200, `{}`
	if err := c.Cancel(ctx, "SP1P01", "req-1", "dibatalkan pembeli"); err != nil || gotPath != "POST /checkout/v3/cancellations" {
		t.Fatalf("cancel: %v %s", err, gotPath)
	}
	o, _ := gotBody["order"].(map[string]any)
	p, _ := gotBody["payment"].(map[string]any)
	if o["invoice_number"] != "SP1P01" || p["original_request_id"] != "req-1" || gotBody["note"] == "" {
		t.Fatalf("body cancel salah: %v", gotBody)
	}

	httpStatus, reply = 400, `{"error":{"message":"Transaction cannot be cancelled"}}`
	var apiErr *APIError
	if err := c.Cancel(ctx, "SP1P01", "req-1", "x"); !errors.As(err, &apiErr) || !apiErr.Rejected() || apiErr.Messages[0] != "Transaction cannot be cancelled" {
		t.Fatalf("cancel ditolak: %v", err)
	}

	// Client ID / secret salah → bukan "ditolak karena keadaan order".
	httpStatus, reply = 401, `{"error":{"message":"Invalid Signature"}}`
	if _, err := c.GetStatus(ctx, "SP1P01"); !errors.As(err, &apiErr) || apiErr.Rejected() {
		t.Fatalf("401: %v", err)
	}
}

func TestVerifyNotification(t *testing.T) {
	body := []byte(`{"order":{"invoice_number":"SP1P01","amount":95000},"transaction":{"status":"SUCCESS"}}`)
	path := "/api/v1/webhooks/doku"
	h := http.Header{}
	h.Set("Client-Id", "BRN-1")
	h.Set("Request-Id", "n-1")
	h.Set("Request-Timestamp", "2026-11-16T03:00:00Z")
	h.Set("Signature", Sign("BRN-1", "n-1", "2026-11-16T03:00:00Z", path, body, "sk"))
	if err := VerifyNotification(h, path, body, "BRN-1", "sk"); err != nil {
		t.Fatalf("notifikasi valid ditolak: %v", err)
	}
	if VerifyNotification(h, path, body, "BRN-1", "sk-lain") == nil {
		t.Fatal("secret lain diterima")
	}
	if VerifyNotification(h, path, []byte(`{"order":{"invoice_number":"SP1P01","amount":1}}`), "BRN-1", "sk") == nil {
		t.Fatal("body diubah tapi diterima")
	}
	if VerifyNotification(h, "/lain", body, "BRN-1", "sk") == nil {
		t.Fatal("path lain diterima")
	}
	if VerifyNotification(h, path, body, "BRN-2", "sk") == nil {
		t.Fatal("Client-Id lain diterima")
	}
	h.Del("Request-Id")
	if VerifyNotification(h, path, body, "BRN-1", "sk") == nil {
		t.Fatal("tanpa Request-Id diterima")
	}
}

func TestAmountRupiah(t *testing.T) {
	for in, want := range map[string]int64{"95000": 95000, "95000.00": 95000, "95000.0": 95000} {
		if got, err := (&Status{Order: OrderInfo{Amount: Amount(in)}}).AmountRupiah(); err != nil || got != want {
			t.Errorf("%s → %d %v", in, got, err)
		}
	}
	for _, in := range []string{"95000.50", "", "abc", ".00"} {
		if _, err := (&Status{Order: OrderInfo{Amount: Amount(in)}}).AmountRupiah(); err == nil {
			t.Errorf("%q harus error", in)
		}
	}
	// Amount ditulis sebagai angka (body notifikasi buatan server palsu).
	b, _ := json.Marshal(OrderInfo{InvoiceNumber: "X", Amount: "95000"})
	if string(b) != `{"invoice_number":"X","amount":95000}` {
		t.Fatalf("marshal amount: %s", b)
	}
}
