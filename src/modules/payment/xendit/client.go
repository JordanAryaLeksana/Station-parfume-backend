// Package xendit adalah client kecil untuk Xendit Payment Sessions
// (POST /sessions, GET /sessions/{id}, POST /sessions/{id}/cancel).
// Hanya net/http: SDK resmi (xendit-go v7) tidak mendukung Payment Sessions.
//
// Dokumentasi (dicek 2026-11-10):
//   - https://docs.xendit.co/apidocs/create-session
//   - https://docs.xendit.co/apidocs/get-session
//   - https://docs.xendit.co/apidocs/cancel-session
//   - https://docs.xendit.co/apidocs/webhook-notification-sent-defined-webhook-url-updates-payment-session
package xendit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.xendit.co"

// Status sesi.
const (
	StatusActive    = "ACTIVE"
	StatusCompleted = "COMPLETED"
	StatusExpired   = "EXPIRED"
	StatusCanceled  = "CANCELED"
)

// Event webhook sesi.
const (
	EventSessionCompleted = "payment_session.completed"
	EventSessionExpired   = "payment_session.expired"
)

type Item struct {
	ReferenceID   string `json:"reference_id"`
	Type          string `json:"type"` // PHYSICAL_PRODUCT
	Name          string `json:"name"`
	NetUnitAmount int64  `json:"net_unit_amount"`
	Quantity      int    `json:"quantity"`
	Category      string `json:"category"`
}

type CreateSessionRequest struct {
	ReferenceID      string            `json:"reference_id"`
	Currency         string            `json:"currency"`     // IDR
	Amount           int64             `json:"amount"`       // rupiah utuh
	Country          string            `json:"country"`      // ID
	SessionType      string            `json:"session_type"` // PAY
	Mode             string            `json:"mode"`         // PAYMENT_LINK
	Items            []Item            `json:"items,omitempty"`
	ExpiresAt        *time.Time        `json:"expires_at,omitempty"` // min 10 menit dari sekarang
	SuccessReturnURL string            `json:"success_return_url,omitempty"`
	CancelReturnURL  string            `json:"cancel_return_url,omitempty"`
	Description      string            `json:"description,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// Session = respons create/get/cancel.
type Session struct {
	PaymentSessionID string      `json:"payment_session_id"`
	ReferenceID      string      `json:"reference_id"`
	Status           string      `json:"status"`
	PaymentLinkURL   string      `json:"payment_link_url"`
	ExpiresAt        *time.Time  `json:"expires_at"`
	PaymentID        string      `json:"payment_id"`
	PaymentRequestID string      `json:"payment_request_id"`
	Amount           json.Number `json:"amount"`
	Currency         string      `json:"currency"`
	Raw              []byte      `json:"-"` // body respons asli (disimpan di payments.provider_response)
}

// AmountRupiah: amount sebagai rupiah utuh; error kalau bukan bilangan bulat.
func (s *Session) AmountRupiah() (int64, error) {
	str := s.Amount.String()
	if v, err := strconv.ParseInt(str, 10, 64); err == nil {
		return v, nil
	}
	f, err := strconv.ParseFloat(str, 64)
	if err != nil || f != float64(int64(f)) {
		return 0, fmt.Errorf("amount %q bukan rupiah utuh", str)
	}
	return int64(f), nil
}

// WebhookEvent = body webhook sesi. Isinya TIDAK dipercaya: status selalu
// dicek ulang lewat GetSession.
type WebhookEvent struct {
	Event      string `json:"event"`
	BusinessID string `json:"business_id"`
	Data       struct {
		PaymentSessionID string `json:"payment_session_id"`
		ReferenceID      string `json:"reference_id"`
		Status           string `json:"status"`
		PaymentID        string `json:"payment_id"`
	} `json:"data"`
}

// APIError = respons error Xendit (non-2xx).
type APIError struct {
	HTTPStatus int    `json:"-"`
	ErrorCode  string `json:"error_code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("xendit %d %s: %s", e.HTTPStatus, e.ErrorCode, e.Message)
}

type Client struct {
	BaseURL   string
	SecretKey string
	HTTP      *http.Client
}

// New membuat client dengan timeout 10 detik. baseURL kosong = api.xendit.co.
func New(baseURL, secretKey string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), SecretKey: secretKey, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) CreateSession(ctx context.Context, req CreateSessionRequest) (*Session, error) {
	return c.do(ctx, http.MethodPost, "/sessions", req)
}

func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	return c.do(ctx, http.MethodGet, "/sessions/"+url.PathEscape(id), nil)
}

// CancelSession: hanya sesi ACTIVE yang bisa dibatalkan (selain itu 422 INVALID_SESSION_STATUS).
func (c *Client) CancelSession(ctx context.Context, id string) (*Session, error) {
	return c.do(ctx, http.MethodPost, "/sessions/"+url.PathEscape(id)+"/cancel", nil)
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*Session, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	if err != nil {
		return nil, err
	}
	// Basic auth: secret key sebagai username, password kosong.
	req.SetBasicAuth(c.SecretKey, "")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xendit %s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		apiErr := &APIError{HTTPStatus: res.StatusCode}
		_ = json.Unmarshal(raw, apiErr)
		if apiErr.Message == "" {
			apiErr.Message = strings.TrimSpace(string(raw))
		}
		return nil, apiErr
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("xendit %s %s: respons tidak valid: %w", method, path, err)
	}
	s.Raw = raw
	return &s, nil
}
