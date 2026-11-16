// Package doku adalah client kecil untuk DOKU Checkout (hosted payment page),
// API cek status, dan pembatalan order. Hanya net/http, tanpa SDK.
//
// Dokumentasi (dicek 2026-11-16):
//   - Create:  https://developers.doku.com/accept-payments/doku-checkout/integration-guide/backend-integration
//     POST {api}/checkout/v1/payment → response.payment.{url, token_id, expired_date}
//     order.invoice_number maks 64 (maks 30 kalau kartu kredit aktif; tanpa simbol untuk KKI),
//     payment.payment_due_date dalam menit (default 60).
//   - Status:  https://developers.doku.com/get-started-with-doku-api/check-status-api/non-snap
//     GET {api}/orders/v1/status/{invoice_number}. transaction.status:
//     PENDING, SUCCESS (final), FAILED (tidak final: pembeli bisa coba lagi), EXPIRED (final),
//     REFUNDED, TIMEOUT, REDIRECT. Order Checkout yang belum dipilih channel-nya:
//     transaction.status PENDING + order.status ORDER_GENERATED (akun lama: Not Found).
//   - Cancel:  https://developers.doku.com/accept-payments/doku-checkout/order-and-notification-handling/cancel-order-api
//     POST {api}/checkout/v3/cancellations. Hanya VA & QRIS, harus dinyalakan di dashboard;
//     yang sudah dibayar/expired tidak bisa dibatalkan.
//   - Signature: https://developers.doku.com/get-started-with-doku-api/signature-component/non-snap
//     HMACSHA256=base64(HMAC-SHA256(secret, komponen)); komponen = baris
//     Client-Id, Request-Id, Request-Timestamp, Request-Target (+ Digest = base64(SHA256(body))
//     hanya untuk POST), dipisah "\n". Notifikasi ke kita memakai skema yang sama dengan
//     Request-Target = path URL notifikasi kita.
//   - Notifikasi: wajib dijawab 2xx; diulang 3× (30 menit, 6 jam, 12 jam).
package doku

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SandboxURL    = "https://api-sandbox.doku.com"
	ProductionURL = "https://api.doku.com"

	PathCreate = "/checkout/v1/payment"
	PathStatus = "/orders/v1/status/"
	PathCancel = "/checkout/v3/cancellations"
)

// transaction.status (dinormalisasi huruf besar).
const (
	StatusPending  = "PENDING"
	StatusSuccess  = "SUCCESS"
	StatusFailed   = "FAILED"
	StatusExpired  = "EXPIRED"
	StatusRefunded = "REFUNDED"
	StatusTimeout  = "TIMEOUT"
	StatusRedirect = "REDIRECT"
)

// order.status halaman Checkout.
const (
	OrderGenerated = "ORDER_GENERATED"
	OrderExpired   = "ORDER_EXPIRED"
	OrderRecovered = "ORDER_RECOVERED"
)

// ErrTransactionNotFound: DOKU belum mengenal invoice ini (akun lama menjawab Not Found
// sampai pembeli memilih channel).
var ErrTransactionNotFound = errors.New("doku: order belum ada di DOKU")

// ErrBadSignature: header notifikasi tidak lolos verifikasi.
var ErrBadSignature = errors.New("doku: signature notifikasi tidak valid")

// WIB: zona waktu expired_date di respons DOKU.
var WIB = time.FixedZone("WIB", 7*60*60)

// TimestampLayout: format Request-Timestamp (ISO 8601, UTC).
const TimestampLayout = "2006-01-02T15:04:05Z"

type LineItem struct {
	ID       string `json:"id,omitempty"` // maks 64
	Name     string `json:"name"`         // maks 255
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
	Category string `json:"category,omitempty"`
}

type OrderRequest struct {
	Amount            int64      `json:"amount"`
	InvoiceNumber     string     `json:"invoice_number"`
	Currency          string     `json:"currency,omitempty"`
	CallbackURL       string     `json:"callback_url,omitempty"`        // tombol "kembali ke toko"
	CallbackURLResult string     `json:"callback_url_result,omitempty"` // tombol di halaman hasil
	AutoRedirect      bool       `json:"auto_redirect"`                 // true: halaman hasil langsung ke callback
	LineItems         []LineItem `json:"line_items,omitempty"`          // total harus = amount
}

type PaymentOptions struct {
	PaymentDueDate int `json:"payment_due_date,omitempty"` // menit
}

type Customer struct {
	Name  string `json:"name,omitempty"`  // maks 255
	Email string `json:"email,omitempty"` // maks 128
}

type PaymentRequest struct {
	Order    OrderRequest   `json:"order"`
	Payment  PaymentOptions `json:"payment"`
	Customer *Customer      `json:"customer,omitempty"`
}

// PaymentResponse = hasil create Checkout.
type PaymentResponse struct {
	URL         string // halaman bayar DOKU
	TokenID     string
	ExpiredDate string // yyyyMMddHHmmss, WIB
	RequestID   string // Request-Id create (dibutuhkan untuk cancel)
	Raw         []byte // body respons asli (payments.provider_response)
}

// Amount menerima angka maupun string ("20000", "20000.00") karena DOKU tidak konsisten.
type Amount string

func (a *Amount) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch {
	case s == "null":
		*a = ""
	case strings.HasPrefix(s, `"`):
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*a = Amount(v)
	default:
		*a = Amount(s)
	}
	return nil
}

// MarshalJSON menulis angka kalau isinya angka (dipakai server palsu di tes).
func (a Amount) MarshalJSON() ([]byte, error) {
	if _, err := strconv.ParseFloat(string(a), 64); err == nil {
		return []byte(a), nil
	}
	return json.Marshal(string(a))
}

type OrderInfo struct {
	InvoiceNumber string `json:"invoice_number"`
	Amount        Amount `json:"amount"`
	Currency      string `json:"currency,omitempty"`
	Status        string `json:"status,omitempty"` // ORDER_GENERATED / ORDER_EXPIRED / ORDER_RECOVERED
}

type TransactionInfo struct {
	Status            string `json:"status"`
	Date              string `json:"date,omitempty"`
	OriginalRequestID string `json:"original_request_id,omitempty"`
}

type Ref struct {
	ID string `json:"id"`
}

// Status = respons cek status, dan juga body notifikasi (webhook). Isi notifikasi
// TIDAK dipercaya: status selalu dicek ulang lewat GetStatus.
type Status struct {
	Order       OrderInfo       `json:"order"`
	Transaction TransactionInfo `json:"transaction"`
	Service     Ref             `json:"service"`  // mis. VIRTUAL_ACCOUNT, QRIS, EMONEY
	Acquirer    Ref             `json:"acquirer"` // mis. BRI, SHOPEE_PAY
	Channel     Ref             `json:"channel"`  // mis. VIRTUAL_ACCOUNT_BRI, EMONEY_SHOPEE_PAY
	Raw         []byte          `json:"-"`
}

// TxStatus: transaction.status huruf besar.
func (s *Status) TxStatus() string { return strings.ToUpper(strings.TrimSpace(s.Transaction.Status)) }

// OrderStatus: order.status huruf besar.
func (s *Status) OrderStatus() string { return strings.ToUpper(strings.TrimSpace(s.Order.Status)) }

// Paid: transaksi sudah dibayar pembeli.
func (s *Status) Paid() bool { return s.TxStatus() == StatusSuccess }

// Expired: batas bayar lewat (transaksi atau halaman Checkout-nya).
func (s *Status) Expired() bool {
	return s.TxStatus() == StatusExpired || (s.OrderStatus() == OrderExpired && !s.Paid())
}

// Method: jenis layanan (service.id) untuk payments.payment_method.
func (s *Status) Method() string { return strings.ToUpper(strings.TrimSpace(s.Service.ID)) }

// ChannelName: channel.id (fallback acquirer.id) untuk payments.payment_channel.
func (s *Status) ChannelName() string {
	if c := strings.TrimSpace(s.Channel.ID); c != "" {
		return strings.ToUpper(c)
	}
	return strings.ToUpper(strings.TrimSpace(s.Acquirer.ID))
}

// AmountRupiah: order.amount sebagai rupiah utuh; error kalau ada sen.
func (s *Status) AmountRupiah() (int64, error) {
	v := strings.TrimSpace(string(s.Order.Amount))
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n, nil
	}
	if whole, frac, ok := strings.Cut(v, "."); ok && whole != "" && strings.Trim(frac, "0") == "" {
		if n, err := strconv.ParseInt(whole, 10, 64); err == nil {
			return n, nil
		}
	}
	return 0, fmt.Errorf("order.amount %q bukan rupiah utuh", s.Order.Amount)
}

// Sign menghitung header Signature. body nil = tanpa Digest (request GET).
func Sign(clientID, requestID, timestamp, target string, body []byte, secret string) string {
	comp := "Client-Id:" + clientID + "\nRequest-Id:" + requestID + "\nRequest-Timestamp:" + timestamp +
		"\nRequest-Target:" + target
	if body != nil {
		sum := sha256.Sum256(body)
		comp += "\nDigest:" + base64.StdEncoding.EncodeToString(sum[:])
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(comp))
	return "HMACSHA256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyNotification memeriksa header notifikasi DOKU: Client-Id harus milik kita
// dan Signature dihitung ulang dari header + path + body (constant-time).
func VerifyNotification(h http.Header, path string, body []byte, clientID, secret string) error {
	reqID, ts, sig := h.Get("Request-Id"), h.Get("Request-Timestamp"), h.Get("Signature")
	if clientID == "" || secret == "" || h.Get("Client-Id") != clientID || reqID == "" || ts == "" || sig == "" {
		return ErrBadSignature
	}
	if body == nil {
		body = []byte{}
	}
	want := Sign(clientID, reqID, ts, path, body, secret)
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return ErrBadSignature
	}
	return nil
}

// APIError = respons error DOKU.
type APIError struct {
	HTTPStatus int
	Messages   []string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("doku http %d: %s", e.HTTPStatus, strings.Join(e.Messages, "; "))
}

// Rejected: DOKU menolak karena keadaan order/data (4xx selain 401/403 yang berarti
// Client ID / Secret Key / signature salah).
func (e *APIError) Rejected() bool {
	c := e.HTTPStatus
	return c >= 400 && c < 500 && c != http.StatusUnauthorized && c != http.StatusForbidden
}

type Client struct {
	APIURL    string
	ClientID  string
	SecretKey string
	HTTP      *http.Client
	now       func() time.Time
}

// New membuat client dengan timeout 10 detik. apiURL kosong = URL sandbox atau
// production sesuai `production`.
func New(clientID, secretKey string, production bool, apiURL string) *Client {
	if apiURL == "" {
		apiURL = SandboxURL
		if production {
			apiURL = ProductionURL
		}
	}
	return &Client{APIURL: strings.TrimRight(apiURL, "/"), ClientID: clientID, SecretKey: secretKey,
		HTTP: &http.Client{Timeout: 10 * time.Second}, now: time.Now}
}

// CreatePayment membuat halaman DOKU Checkout untuk satu invoice.
func (c *Client) CreatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	res, raw, reqID, err := c.send(ctx, http.MethodPost, PathCreate, req)
	if err != nil {
		return nil, err
	}
	var body struct {
		Response struct {
			Payment struct {
				URL         string `json:"url"`
				TokenID     string `json:"token_id"`
				ExpiredDate string `json:"expired_date"`
			} `json:"payment"`
		} `json:"response"`
	}
	jsonErr := json.Unmarshal(raw, &body)
	p := body.Response.Payment
	if res.StatusCode < 200 || res.StatusCode > 299 || jsonErr != nil || p.URL == "" || p.TokenID == "" {
		return nil, apiError(res.StatusCode, raw)
	}
	return &PaymentResponse{URL: p.URL, TokenID: p.TokenID, ExpiredDate: p.ExpiredDate, RequestID: reqID, Raw: raw}, nil
}

// GetStatus: status order berdasarkan invoice_number.
func (c *Client) GetStatus(ctx context.Context, invoice string) (*Status, error) {
	res, raw, _, err := c.send(ctx, http.MethodGet, PathStatus+url.PathEscape(invoice), nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrTransactionNotFound
	}
	var s Status
	jsonErr := json.Unmarshal(raw, &s)
	if res.StatusCode < 200 || res.StatusCode > 299 || jsonErr != nil {
		return nil, apiError(res.StatusCode, raw)
	}
	if s.Order.InvoiceNumber == "" && s.Transaction.Status == "" {
		return nil, ErrTransactionNotFound
	}
	s.Raw = raw
	return &s, nil
}

// Cancel membatalkan order Checkout yang belum dibayar (hanya VA & QRIS; fitur
// harus dinyalakan di dashboard). Ditolak → *APIError dengan Rejected() true.
func (c *Client) Cancel(ctx context.Context, invoice, originalRequestID, note string) error {
	body := map[string]any{
		"order":   map[string]string{"invoice_number": invoice},
		"payment": map[string]string{"original_request_id": originalRequestID},
		"note":    note,
	}
	res, raw, _, err := c.send(ctx, http.MethodPost, PathCancel, body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return apiError(res.StatusCode, raw)
	}
	return nil
}

// apiError mengambil pesan error dari beberapa bentuk respons DOKU.
func apiError(status int, raw []byte) *APIError {
	var body struct {
		ErrorMessages []string        `json:"error_messages"`
		Message       json.RawMessage `json:"message"`
		Error         struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &body)
	msgs := body.ErrorMessages
	if len(msgs) == 0 && body.Error.Message != "" {
		msgs = []string{body.Error.Message}
	}
	if len(msgs) == 0 && len(body.Message) > 0 {
		var list []string
		var one string
		if json.Unmarshal(body.Message, &list) == nil {
			msgs = list
		} else if json.Unmarshal(body.Message, &one) == nil && one != "" {
			msgs = []string{one}
		}
	}
	if len(msgs) == 0 {
		s := strings.TrimSpace(string(raw))
		if len(s) > 300 {
			s = s[:300]
		}
		msgs = []string{s}
	}
	return &APIError{HTTPStatus: status, Messages: msgs}
}

func (c *Client) send(ctx context.Context, method, path string, body any) (*http.Response, []byte, string, error) {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, "", err
		}
		payload = b
	}
	req, err := http.NewRequestWithContext(ctx, method, c.APIURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, "", err
	}
	reqID := uuid.NewString()
	ts := c.now().UTC().Format(TimestampLayout)
	// Request-Target = path yang benar-benar dikirim (sudah di-escape).
	req.Header.Set("Client-Id", c.ClientID)
	req.Header.Set("Request-Id", reqID)
	req.Header.Set("Request-Timestamp", ts)
	req.Header.Set("Signature", Sign(c.ClientID, reqID, ts, req.URL.EscapedPath(), payload, c.SecretKey))
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, "", fmt.Errorf("doku %s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, nil, "", err
	}
	return res, raw, reqID, nil
}
