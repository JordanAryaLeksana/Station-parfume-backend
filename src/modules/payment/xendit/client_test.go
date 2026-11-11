package xendit

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

func TestCreateGetCancelSession(t *testing.T) {
	var gotAuth, gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotMethod = r.Header.Get("Authorization"), r.URL.Path, r.Method
		gotBody = nil
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &gotBody)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payment_session_id":"ps-123","reference_id":"ref-1","status":"ACTIVE","amount":95000,"currency":"IDR",
			"payment_link_url":"https://checkout-staging.xendit.co/sessions/ps-123","expires_at":"2026-11-11T10:00:00Z"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "xnd_development_secret")
	exp := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	s, err := c.CreateSession(context.Background(), CreateSessionRequest{ReferenceID: "ref-1", Currency: "IDR", Amount: 95000, Country: "ID",
		SessionType: "PAY", Mode: "PAYMENT_LINK", ExpiresAt: &exp, Items: []Item{{ReferenceID: "v", Type: "PHYSICAL_PRODUCT", Name: "Annasui", NetUnitAmount: 95000, Quantity: 1, Category: "Parfum"}}})
	if err != nil {
		t.Fatal(err)
	}
	// Basic auth: secret key sebagai username, password kosong.
	if want := "Basic " + base64.StdEncoding.EncodeToString([]byte("xnd_development_secret:")); gotAuth != want {
		t.Fatalf("auth = %q, want %q", gotAuth, want)
	}
	if gotMethod != "POST" || gotPath != "/sessions" || gotBody["amount"] != float64(95000) || gotBody["session_type"] != "PAY" ||
		gotBody["mode"] != "PAYMENT_LINK" || gotBody["expires_at"] != "2026-11-11T10:00:00Z" || len(gotBody["items"].([]any)) != 1 {
		t.Fatalf("request salah: %s %s %v", gotMethod, gotPath, gotBody)
	}
	if s.PaymentSessionID != "ps-123" || s.PaymentLinkURL == "" || s.ExpiresAt == nil || len(s.Raw) == 0 {
		t.Fatalf("respons salah: %+v", s)
	}
	if a, err := s.AmountRupiah(); err != nil || a != 95000 {
		t.Fatalf("amount: %d %v", a, err)
	}
	if _, err := c.GetSession(context.Background(), "ps-123"); err != nil || gotMethod != "GET" || gotPath != "/sessions/ps-123" {
		t.Fatalf("get: %v %s %s", err, gotMethod, gotPath)
	}
	if _, err := c.CancelSession(context.Background(), "ps-123"); err != nil || gotMethod != "POST" || gotPath != "/sessions/ps-123/cancel" {
		t.Fatalf("cancel: %v %s %s", err, gotMethod, gotPath)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"error_code":"INVALID_SESSION_STATUS","message":"session is not active"}`))
	}))
	defer srv.Close()
	_, err := New(srv.URL, "k").CancelSession(context.Background(), "ps-1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.HTTPStatus != 422 || apiErr.ErrorCode != "INVALID_SESSION_STATUS" {
		t.Fatalf("error salah: %v", err)
	}
}

func TestAmountRupiah(t *testing.T) {
	for in, want := range map[string]int64{"95000": 95000, "95000.0": 95000} {
		s := Session{Amount: json.Number(in)}
		if got, err := s.AmountRupiah(); err != nil || got != want {
			t.Errorf("%s → %d %v", in, got, err)
		}
	}
	for _, in := range []string{"95000.5", "", "abc"} {
		s := Session{Amount: json.Number(in)}
		if _, err := s.AmountRupiah(); err == nil {
			t.Errorf("%q harus error", in)
		}
	}
}
