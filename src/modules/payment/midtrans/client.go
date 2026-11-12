// Package midtrans adalah client kecil untuk Midtrans Snap (hosted checkout)
// dan API status/expire transaksi. Hanya net/http, tanpa SDK.
//
// Dokumentasi (dicek 2026-11-12):
//   - Snap create:  https://docs.midtrans.com/reference/backend-integration
//     POST {app}/snap/v1/transactions → 201 {token, redirect_url}
//   - Expiry:       https://docs.midtrans.com/docs/snap-advanced-feature
//     expiry.start_time "YYYY-MM-DD HH:MM:SS +0700"; token & redirect_url default 24 jam
//   - Status:       https://docs.midtrans.com/reference/get-transaction-status
//     GET {api}/v2/{order_id|transaction_id}/status. Untuk DANA/BI SNAP hanya
//     transaction_id yang berfungsi.
//   - Expire:       https://docs.midtrans.com/reference/expire-transaction
//     POST {api}/v2/{id}/expire. Berlaku untuk Snap; sukses = status_code "407";
//     transaksi yang tidak bisa diubah = "412". order_id yang di-expire bisa dipakai
//     lagi, jadi link Snap yang belum habis masih bisa dibayar.
//   - Notifikasi:   https://docs.midtrans.com/docs/https-notification-webhooks
//     signature_key = SHA512(order_id+status_code+gross_amount+ServerKey);
//     jawab HTTP 200, cek ulang lewat GET status.
//
// API v2 (status/expire) menjawab HTTP 200 juga untuk banyak error: hasil
// sebenarnya ada di field status_code di body.
package midtrans

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	SandboxSnapURL    = "https://app.sandbox.midtrans.com"
	ProductionSnapURL = "https://app.midtrans.com"
	SandboxAPIURL     = "https://api.sandbox.midtrans.com"
	ProductionAPIURL  = "https://api.midtrans.com"
)

// transaction_status (dinormalisasi huruf kecil; DANA bisa mengirim "PENDING").
const (
	StatusPending       = "pending"
	StatusSettlement    = "settlement"
	StatusCapture       = "capture"
	StatusAuthorize     = "authorize"
	StatusDeny          = "deny"
	StatusCancel        = "cancel"
	StatusExpire        = "expire"
	StatusFailure       = "failure"
	StatusRefund        = "refund"
	StatusPartialRefund = "partial_refund"
)

// fraud_status (kartu). Pembayaran capture baru dianggap lunas kalau accept.
const (
	FraudAccept    = "accept"
	FraudChallenge = "challenge"
	FraudDeny      = "deny"
)

// ErrTransactionNotFound: Midtrans belum punya transaksi untuk id ini. Pada Snap,
// transaksi baru ada setelah pembeli memilih metode bayar di halaman Snap.
var ErrTransactionNotFound = errors.New("midtrans: transaksi belum ada (pembeli belum memilih metode bayar)")

// WIB: zona waktu yang diminta Midtrans untuk expiry.start_time.
var WIB = time.FixedZone("WIB", 7*60*60)

// FormatTime: format waktu untuk expiry.start_time ("2026-11-12 10:26:00 +0700").
func FormatTime(t time.Time) string { return t.In(WIB).Format("2006-01-02 15:04:05 -0700") }

type TransactionDetails struct {
	OrderID     string `json:"order_id"`     // maks 50 karakter
	GrossAmount int64  `json:"gross_amount"` // rupiah utuh = jumlah price×quantity item
}

type Item struct {
	ID       string `json:"id,omitempty"` // maks 50 karakter
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
	Name     string `json:"name"` // maks 50 karakter
	Category string `json:"category,omitempty"`
}

type Callbacks struct {
	Finish string `json:"finish,omitempty"` // Midtrans menambahkan ?order_id=&status_code=&transaction_status=
}

// Expiry: batas bayar transaksi, dihitung dari StartTime.
type Expiry struct {
	StartTime string `json:"start_time,omitempty"`
	Unit      string `json:"unit"` // minute | hour | day
	Duration  int    `json:"duration"`
}

// PageExpiry: umur halaman Snap (token & redirect_url).
type PageExpiry struct {
	Duration int    `json:"duration"`
	Unit     string `json:"unit"`
}

type SnapRequest struct {
	TransactionDetails TransactionDetails `json:"transaction_details"`
	ItemDetails        []Item             `json:"item_details,omitempty"`
	Callbacks          *Callbacks         `json:"callbacks,omitempty"`
	Expiry             *Expiry            `json:"expiry,omitempty"`
	PageExpiry         *PageExpiry        `json:"page_expiry,omitempty"`
	CustomField1       string             `json:"custom_field1,omitempty"` // nomor pesanan, tampil di Dashboard
}

// SnapResponse = respons create Snap.
type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
	Raw         []byte `json:"-"` // body respons asli (disimpan di payments.provider_response)
}

type VANumber struct {
	Bank     string `json:"bank"`
	VANumber string `json:"va_number"`
}

// Transaction = respons GET status / expire, dan juga body notifikasi (webhook).
// Isi notifikasi TIDAK dipercaya: status selalu dicek ulang lewat GetStatus.
type Transaction struct {
	StatusCode        string     `json:"status_code"`
	StatusMessage     string     `json:"status_message"`
	TransactionID     string     `json:"transaction_id"`
	OrderID           string     `json:"order_id"`
	GrossAmount       string     `json:"gross_amount"` // "100000.00"
	Currency          string     `json:"currency"`
	PaymentType       string     `json:"payment_type"` // qris, gopay, bank_transfer, echannel, ...
	TransactionStatus string     `json:"transaction_status"`
	FraudStatus       string     `json:"fraud_status"`
	SignatureKey      string     `json:"signature_key"`
	Bank              string     `json:"bank"`
	Issuer            string     `json:"issuer"`
	Acquirer          string     `json:"acquirer"`
	VANumbers         []VANumber `json:"va_numbers"`
	PermataVANumber   string     `json:"permata_va_number"`
	BillerCode        string     `json:"biller_code"` // Mandiri Bill (echannel)
	Raw               []byte     `json:"-"`
}

// Status: transaction_status huruf kecil.
func (t *Transaction) Status() string { return strings.ToLower(strings.TrimSpace(t.TransactionStatus)) }

// Fraud: fraud_status huruf kecil.
func (t *Transaction) Fraud() string { return strings.ToLower(strings.TrimSpace(t.FraudStatus)) }

// Paid: settlement, atau capture kartu yang lolos FDS.
func (t *Transaction) Paid() bool {
	s := t.Status()
	return s == StatusSettlement || (s == StatusCapture && t.Fraud() == FraudAccept)
}

// AmountRupiah: gross_amount ("100000.00") sebagai rupiah utuh; error kalau ada sen.
func (t *Transaction) AmountRupiah() (int64, error) {
	s := strings.TrimSpace(t.GrossAmount)
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, nil
	}
	if whole, frac, ok := strings.Cut(s, "."); ok && whole != "" && strings.Trim(frac, "0") == "" {
		if v, err := strconv.ParseInt(whole, 10, 64); err == nil {
			return v, nil
		}
	}
	return 0, fmt.Errorf("gross_amount %q bukan rupiah utuh", t.GrossAmount)
}

// Channel: bank/penerbit untuk payments.payment_channel (mis. BCA, PERMATA, GOPAY).
func (t *Transaction) Channel() string {
	switch {
	case len(t.VANumbers) > 0 && t.VANumbers[0].Bank != "":
		return strings.ToUpper(t.VANumbers[0].Bank)
	case t.PermataVANumber != "":
		return "PERMATA"
	case t.BillerCode != "":
		return "MANDIRI"
	case t.Bank != "":
		return strings.ToUpper(t.Bank)
	case t.Issuer != "":
		return strings.ToUpper(t.Issuer)
	case t.Acquirer != "":
		return strings.ToUpper(t.Acquirer)
	}
	return ""
}

// Signature menghitung signature_key notifikasi:
// SHA512(order_id + status_code + gross_amount + ServerKey), hex huruf kecil.
func Signature(orderID, statusCode, grossAmount, serverKey string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(sum[:])
}

// VerifySignature membandingkan signature_key notifikasi secara constant-time.
func VerifySignature(n *Transaction, serverKey string) bool {
	if serverKey == "" || n.SignatureKey == "" {
		return false
	}
	want := Signature(n.OrderID, n.StatusCode, n.GrossAmount, serverKey)
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(n.SignatureKey)), []byte(want)) == 1
}

// APIError = respons error Midtrans. HTTPStatus = status HTTP; StatusCode =
// status_code di body (API v2 sering menjawab HTTP 200 dengan status_code 4xx).
type APIError struct {
	HTTPStatus int
	StatusCode string
	Messages   []string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("midtrans http %d status %s: %s", e.HTTPStatus, e.StatusCode, strings.Join(e.Messages, "; "))
}

// Code: status_code di body kalau ada, kalau tidak status HTTP.
func (e *APIError) Code() int {
	if n, err := strconv.Atoi(e.StatusCode); err == nil {
		return n
	}
	return e.HTTPStatus
}

// Rejected: Midtrans menolak permintaan karena keadaan transaksi (4xx selain
// 401/403 yang berarti server key salah). Mis. 412 = status tidak bisa diubah.
func (e *APIError) Rejected() bool {
	c := e.Code()
	return c >= 400 && c < 500 && c != http.StatusUnauthorized && c != http.StatusForbidden
}

type Client struct {
	SnapURL   string
	APIURL    string
	ServerKey string
	HTTP      *http.Client
}

// New membuat client dengan timeout 10 detik. snapURL/apiURL kosong = URL
// sandbox atau production sesuai `production`.
func New(serverKey string, production bool, snapURL, apiURL string) *Client {
	if snapURL == "" {
		snapURL = SandboxSnapURL
		if production {
			snapURL = ProductionSnapURL
		}
	}
	if apiURL == "" {
		apiURL = SandboxAPIURL
		if production {
			apiURL = ProductionAPIURL
		}
	}
	return &Client{SnapURL: strings.TrimRight(snapURL, "/"), APIURL: strings.TrimRight(apiURL, "/"),
		ServerKey: serverKey, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// CreateSnap membuat transaksi Snap → token + redirect_url halaman bayar.
func (c *Client) CreateSnap(ctx context.Context, req SnapRequest) (*SnapResponse, error) {
	res, raw, err := c.send(ctx, http.MethodPost, c.SnapURL+"/snap/v1/transactions", req)
	if err != nil {
		return nil, err
	}
	var out SnapResponse
	_ = json.Unmarshal(raw, &out)
	if res.StatusCode < 200 || res.StatusCode > 299 || out.Token == "" || out.RedirectURL == "" {
		var body struct {
			ErrorMessages []string `json:"error_messages"`
		}
		_ = json.Unmarshal(raw, &body)
		if len(body.ErrorMessages) == 0 {
			body.ErrorMessages = []string{strings.TrimSpace(string(raw))}
		}
		return nil, &APIError{HTTPStatus: res.StatusCode, Messages: body.ErrorMessages}
	}
	out.Raw = raw
	return &out, nil
}

// GetStatus: status transaksi berdasarkan order_id atau transaction_id.
// ErrTransactionNotFound kalau pembeli belum memilih metode bayar.
func (c *Client) GetStatus(ctx context.Context, id string) (*Transaction, error) {
	return c.v2(ctx, http.MethodGet, "/v2/"+url.PathEscape(id)+"/status")
}

// Expire mematikan transaksi yang masih pending. Transaksi yang sudah lunas/selesai
// → APIError 412; belum ada transaksi → ErrTransactionNotFound.
func (c *Client) Expire(ctx context.Context, id string) (*Transaction, error) {
	return c.v2(ctx, http.MethodPost, "/v2/"+url.PathEscape(id)+"/expire")
}

func (c *Client) v2(ctx context.Context, method, path string) (*Transaction, error) {
	res, raw, err := c.send(ctx, method, c.APIURL+path, nil)
	if err != nil {
		return nil, err
	}
	var t Transaction
	jsonErr := json.Unmarshal(raw, &t)
	if res.StatusCode == http.StatusNotFound || t.StatusCode == "404" {
		return nil, ErrTransactionNotFound
	}
	code, _ := strconv.Atoi(t.StatusCode)
	ok := (code >= 200 && code <= 299) || code == 407 // 407 = berhasil di-expire
	if res.StatusCode < 200 || res.StatusCode > 299 || jsonErr != nil || !ok {
		msg := t.StatusMessage
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		return nil, &APIError{HTTPStatus: res.StatusCode, StatusCode: t.StatusCode, Messages: []string{msg}}
	}
	t.Raw = raw
	return &t, nil
}

func (c *Client) send(ctx context.Context, method, fullURL string, body any) (*http.Response, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, rd)
	if err != nil {
		return nil, nil, err
	}
	// Basic auth: server key sebagai username, password kosong.
	req.SetBasicAuth(c.ServerKey, "")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("midtrans %s %s: %w", method, req.URL.Path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	return res, raw, nil
}
