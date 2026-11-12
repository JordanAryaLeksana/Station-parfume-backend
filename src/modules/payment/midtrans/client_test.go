package midtrans

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateSnap(t *testing.T) {
	var gotAuth, gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotMethod = r.Header.Get("Authorization"), r.URL.Path, r.Method
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"token":"66e4fa55-fdac-4ef9-91b5-733b97d1b862","redirect_url":"https://app.sandbox.midtrans.com/snap/v4/redirection/66e4fa55"}`))
	}))
	defer srv.Close()
	c := New("SB-Mid-server-abc", false, srv.URL, "")
	start := time.Date(2026, 11, 12, 3, 26, 0, 0, time.UTC)
	s, err := c.CreateSnap(context.Background(), SnapRequest{
		TransactionDetails: TransactionDetails{OrderID: "ref-1", GrossAmount: 95000},
		ItemDetails:        []Item{{ID: "v", Price: 95000, Quantity: 1, Name: "Annasui"}},
		Callbacks:          &Callbacks{Finish: "https://shop.test/bayar/sukses?order=SP1"},
		Expiry:             &Expiry{StartTime: FormatTime(start), Unit: "minute", Duration: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Basic auth: server key sebagai username, password kosong.
	if want := "Basic " + base64.StdEncoding.EncodeToString([]byte("SB-Mid-server-abc:")); gotAuth != want {
		t.Fatalf("auth = %q, want %q", gotAuth, want)
	}
	td, _ := gotBody["transaction_details"].(map[string]any)
	exp, _ := gotBody["expiry"].(map[string]any)
	if gotMethod != "POST" || gotPath != "/snap/v1/transactions" || td["order_id"] != "ref-1" || td["gross_amount"] != float64(95000) ||
		len(gotBody["item_details"].([]any)) != 1 || exp["start_time"] != "2026-11-12 10:26:00 +0700" || exp["duration"] != float64(60) {
		t.Fatalf("request salah: %s %s %v", gotMethod, gotPath, gotBody)
	}
	if s.Token == "" || s.RedirectURL == "" || len(s.Raw) == 0 {
		t.Fatalf("respons salah: %+v", s)
	}
}

func TestCreateSnapError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error_messages":["transaction_details.gross_amount is not equal to the sum of item_details"]}`))
	}))
	defer srv.Close()
	_, err := New("k", false, srv.URL, "").CreateSnap(context.Background(), SnapRequest{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.HTTPStatus != 400 || len(apiErr.Messages) != 1 {
		t.Fatalf("error salah: %v", err)
	}
}

func TestStatusAndExpire(t *testing.T) {
	var gotPath, gotMethod string
	reply := ""
	httpStatus := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(httpStatus)
		_, _ = w.Write([]byte(reply))
	}))
	defer srv.Close()
	c := New("k", false, "", srv.URL)
	ctx := context.Background()

	reply = `{"status_code":"200","transaction_id":"tx-1","order_id":"ref-1","gross_amount":"95000.00","currency":"IDR",
		"payment_type":"bank_transfer","transaction_status":"settlement","fraud_status":"accept","va_numbers":[{"bank":"bca","va_number":"123"}]}`
	tx, err := c.GetStatus(ctx, "ref-1")
	if err != nil || gotMethod != "GET" || gotPath != "/v2/ref-1/status" || !tx.Paid() || tx.Channel() != "BCA" || len(tx.Raw) == 0 {
		t.Fatalf("status: %v %s %s %+v", err, gotMethod, gotPath, tx)
	}
	if a, err := tx.AmountRupiah(); err != nil || a != 95000 {
		t.Fatalf("amount: %d %v", a, err)
	}

	// DANA mengirim status huruf besar.
	reply = `{"status_code":"201","order_id":"ref-1","gross_amount":"95000.00","transaction_status":"PENDING"}`
	if tx, err := c.GetStatus(ctx, "ref-1"); err != nil || tx.Status() != StatusPending || tx.Paid() {
		t.Fatalf("pending: %v %+v", err, tx)
	}

	// Snap belum dipilih metodenya → 404 (di body maupun HTTP).
	reply = `{"status_code":"404","status_message":"Transaction doesn't exist."}`
	if _, err := c.GetStatus(ctx, "ref-1"); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("404 body: %v", err)
	}
	httpStatus = 404
	if _, err := c.GetStatus(ctx, "ref-1"); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("404 http: %v", err)
	}
	httpStatus = 200

	reply = `{"status_code":"407","status_message":"Success, transaction has expired","order_id":"ref-1","gross_amount":"95000.00","transaction_status":"expire"}`
	if tx, err := c.Expire(ctx, "ref-1"); err != nil || gotMethod != "POST" || gotPath != "/v2/ref-1/expire" || tx.Status() != StatusExpire {
		t.Fatalf("expire: %v %s %s %+v", err, gotMethod, gotPath, tx)
	}

	// Sudah lunas → 412 di body walau HTTP 200.
	reply = `{"status_code":"412","status_message":"Merchant cannot modify the status of the transaction"}`
	_, err = c.Expire(ctx, "ref-1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code() != 412 || !apiErr.Rejected() {
		t.Fatalf("412: %v", err)
	}

	// Server key salah → bukan "ditolak karena keadaan transaksi".
	httpStatus, reply = 401, `{"status_code":"401","status_message":"Access denied"}`
	if _, err := c.GetStatus(ctx, "ref-1"); !errors.As(err, &apiErr) || apiErr.Rejected() {
		t.Fatalf("401: %v", err)
	}
}

func TestSignature(t *testing.T) {
	n := &Transaction{OrderID: "ref-1", StatusCode: "200", GrossAmount: "95000.00"}
	n.SignatureKey = Signature(n.OrderID, n.StatusCode, n.GrossAmount, "server-key")
	if len(n.SignatureKey) != 128 || !VerifySignature(n, "server-key") {
		t.Fatalf("signature benar ditolak: %s", n.SignatureKey)
	}
	if VerifySignature(n, "key-lain") || VerifySignature(n, "") {
		t.Fatal("signature dengan key lain/kosong diterima")
	}
	n.GrossAmount = "1.00"
	if VerifySignature(n, "server-key") {
		t.Fatal("nominal diubah tapi signature diterima")
	}
}

func TestAmountRupiah(t *testing.T) {
	for in, want := range map[string]int64{"95000": 95000, "95000.00": 95000, "95000.0": 95000} {
		if got, err := (&Transaction{GrossAmount: in}).AmountRupiah(); err != nil || got != want {
			t.Errorf("%s → %d %v", in, got, err)
		}
	}
	for _, in := range []string{"95000.50", "", "abc", ".00"} {
		if _, err := (&Transaction{GrossAmount: in}).AmountRupiah(); err == nil {
			t.Errorf("%q harus error", in)
		}
	}
}

func TestPaid(t *testing.T) {
	cases := map[[2]string]bool{
		{"settlement", ""}: true, {"capture", "accept"}: true, {"capture", "challenge"}: false,
		{"pending", ""}: false, {"expire", ""}: false, {"deny", "deny"}: false,
	}
	for c, want := range cases {
		if got := (&Transaction{TransactionStatus: c[0], FraudStatus: c[1]}).Paid(); got != want {
			t.Errorf("%v → %v, want %v", c, got, want)
		}
	}
}
