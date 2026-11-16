//go:build integration

// Package integration: test HTTP end-to-end dengan PostgreSQL + Redis asli.
// Jalankan lewat scripts/test-integration.sh (menyalakan container sementara),
// atau manual dengan DATABASE_URL & REDIS_ADDR menunjuk ke DB KOSONG khusus test:
//
//	go test -tags=integration ./tests/integration/...
//
// PERINGATAN: database di DATABASE_URL dihapus total (DROP SCHEMA public).
// Setiap response divalidasi terhadap api/openapi.yaml.
package integration

import (
	"backend/api"
	"backend/migrations"
	"backend/src/config"
	"backend/src/config/seeders"
	"backend/src/modules/payment/doku"
	paymentservices "backend/src/modules/payment/services"
	"backend/src/routes"
	authhandler "backend/src/utils/AuthHandler"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	frontendOrigin = "https://shop.test"
	dokuClientID   = "BRN-0001-TEST"
	dokuSecret     = "SK-test-rahasia"
	ownerEmail     = "owner@test.local"
	ownerPassword  = "rahasia-panjang-123"
	buyerPassword  = "rahasia-123"
)

var (
	srv      *httptest.Server
	specDoc  *openapi3.T
	oaRouter routers.Router
	fake     = newFakeDoku()
)

func TestMain(m *testing.M) {
	dsn, redisAddr := os.Getenv("DATABASE_URL"), os.Getenv("REDIS_ADDR")
	if dsn == "" || redisAddr == "" {
		fmt.Println("DATABASE_URL dan REDIS_ADDR wajib (pakai scripts/test-integration.sh)")
		os.Exit(1)
	}
	gin.SetMode(gin.TestMode)
	log.SetOutput(io.Discard) // log akses/aplikasi tidak perlu di output test
	os.Setenv("JWT_SECRET", strings.Repeat("a", 40))
	os.Setenv("JWT_REFRESH_SECRET", strings.Repeat("r", 40))
	os.Setenv("DB_LOG_LEVEL", "silent")
	must(config.LoadAuthConfig())
	must(config.InitRedis())
	db, err := config.OpenDatabase(dsn)
	must(err)
	config.DB = db
	sqlDB, err := db.DB()
	must(err)
	ctx := context.Background()
	_, err = sqlDB.ExecContext(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	must(err)
	p, err := migrations.NewProvider(sqlDB)
	must(err)
	_, err = p.Up(ctx)
	must(err)

	specDoc, err = openapi3.NewLoader().LoadFromData(api.OpenAPI)
	must(err)
	must(specDoc.Validate(ctx))

	google := fakeGoogle()
	defer google.Close()
	os.Setenv("GOOGLE_CLIENT_ID", "cid")
	os.Setenv("GOOGLE_CLIENT_SECRET", "csecret")
	os.Setenv("GOOGLE_REDIRECT_URL", "https://api.shop.test/api/v1/auth/google/callback")
	os.Setenv("FRONTEND_URL", frontendOrigin+"/")
	authhandler.InitGoogleOauthConfig()
	authhandler.GoogleOauthConfig.Endpoint = oauth2.Endpoint{AuthURL: google.URL + "/auth", TokenURL: google.URL + "/token", AuthStyle: oauth2.AuthStyleInParams}
	authhandler.UserInfoURL = google.URL + "/userinfo"

	paymentservices.Configure(fake, paymentSettings)

	engine, err := routes.NewEngine(routes.Options{CORSOrigins: []string{frontendOrigin}, Docs: true})
	must(err)
	srv = httptest.NewServer(routes.Handler(engine))
	defer srv.Close()

	// Spec memakai server relatif "/"; router validasi diarahkan ke server test.
	routerDoc := *specDoc
	routerDoc.Servers = openapi3.Servers{{URL: srv.URL}}
	oaRouter, err = gorillamux.NewRouter(&routerDoc)
	must(err)

	code := m.Run()
	srv.Close()
	os.Exit(code)
}

// paymentSettings: konfigurasi pembayaran untuk test (cek ulang status tanpa jeda panjang).
var paymentSettings = paymentservices.Settings{ClientID: dokuClientID, SecretKey: dokuSecret,
	SuccessURL: frontendOrigin + "/bayar/sukses", RecheckDelay: time.Millisecond}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// resetData mengosongkan semua tabel + Redis, lalu seed katalog & akun owner.
func resetData(t *testing.T) {
	t.Helper()
	tables := []string{"payment_events", "payments", "stock_movements", "order_items", "orders", "cart_items", "carts",
		"variants", "product_sizes", "sizes", "product_images", "scents", "products", "categories", "users"}
	if err := config.DB.Exec("TRUNCATE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatal(err)
	}
	if err := config.RedisClient.FlushDB(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	fake.reset()
	dir := filepath.Join("..", "..", "seed")
	var data seeders.CatalogData
	var err error
	load := func(name string, fn func(io.Reader) error) {
		f, e := os.Open(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if e := fn(f); e != nil {
			t.Fatal(e)
		}
	}
	load("categories.csv", func(r io.Reader) error { data.Categories, err = seeders.LoadCategories(r); return err })
	load("sizes.csv", func(r io.Reader) error { data.Sizes, err = seeders.LoadSizes(r); return err })
	load("wangian.csv", func(r io.Reader) error { data.Scents, err = seeders.LoadScents(r); return err })
	load("produk_umum.csv", func(r io.Reader) error { data.General, err = seeders.LoadGeneralProducts(r); return err })
	if _, err := seeders.SeedCatalog(config.DB, data, false); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_NAME", "Owner")
	t.Setenv("ADMIN_EMAIL", ownerEmail)
	t.Setenv("ADMIN_PASSWORD", ownerPassword)
	if _, err := seeders.SeedAdmin(config.DB); err != nil {
		t.Fatal(err)
	}
}

// ------------------------------------------------------------------ klien HTTP

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

type resp struct {
	Status int
	Body   map[string]any
	Raw    []byte
	Header http.Header
	HTTP   *http.Response
}

// Data mengembalikan field "data" dari response sukses.
func (r resp) Data() map[string]any { m, _ := r.Body["data"].(map[string]any); return m }

// List mengembalikan field "data" berupa array.
func (r resp) List() []any { l, _ := r.Body["data"].([]any); return l }

func (r resp) Message() string { s, _ := r.Body["message"].(string); return s }

// call mengirim request ke server test dan memvalidasi response terhadap spec.
// headers: pasangan key, value.
func call(t *testing.T, method, path, token string, body any, headers ...string) resp {
	t.Helper()
	var raw []byte
	switch b := body.(type) {
	case nil:
	case []byte:
		raw = b
	default:
		raw, _ = json.Marshal(b)
	}
	req, _ := http.NewRequest(method, srv.URL+path, bytes.NewReader(raw))
	if raw != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := noRedirect.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	out := resp{Status: res.StatusCode, Header: res.Header, HTTP: res}
	out.Raw, _ = io.ReadAll(res.Body)
	_ = json.Unmarshal(out.Raw, &out.Body)
	validate(t, method, path, raw, res, out.Raw)
	return out
}

// validate: response harus cocok dengan api/openapi.yaml (status & skema).
func validate(t *testing.T, method, path string, reqBody []byte, res *http.Response, body []byte) {
	t.Helper()
	if strings.HasPrefix(path, "/docs") || res.StatusCode == http.StatusNoContent && method == http.MethodOptions {
		return
	}
	req, _ := http.NewRequest(method, srv.URL+strings.TrimSuffix(strings.SplitN(path, "?", 2)[0], "/"), bytes.NewReader(reqBody))
	if u, err := url.Parse(srv.URL + path); err == nil {
		req.URL.RawQuery = u.RawQuery
	}
	route, params, err := oaRouter.FindRoute(req)
	if err != nil {
		if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusMethodNotAllowed {
			return // memang rute tidak ada
		}
		t.Errorf("SPEC: %s %s tidak ada di api/openapi.yaml: %v", method, path, err)
		return
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
		Status:                 res.StatusCode,
		Header:                 res.Header,
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	in.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		msg := err.Error()
		if len(msg) > 600 {
			msg = msg[:600] + "…"
		}
		t.Errorf("SPEC: response %s %s (%d) tidak cocok dengan api/openapi.yaml: %s", method, path, res.StatusCode, msg)
	}
}

func check(t *testing.T, name string, ok bool, info ...any) {
	t.Helper()
	if !ok {
		t.Errorf("GAGAL: %s %s", name, fmt.Sprint(info...))
	}
}

func j(v any) string { b, _ := json.Marshal(v); return string(b) }

func scalar[T any](t *testing.T, q string, args ...any) T {
	t.Helper()
	var v T
	if err := config.DB.Raw(q, args...).Scan(&v).Error; err != nil {
		t.Fatalf("query %q: %v", q, err)
	}
	return v
}

func num(v any) float64 { f, _ := v.(float64); return f }

func login(t *testing.T, path, email, password string) (access, refresh string) {
	t.Helper()
	r := call(t, "POST", path, "", map[string]any{"email": email, "password": password})
	if r.Status != 200 {
		t.Fatalf("login %s: %d %s", email, r.Status, r.Raw)
	}
	d := r.Data()
	return d["access_token"].(string), d["refresh_token"].(string)
}

func ownerToken(t *testing.T) string {
	a, _ := login(t, "/api/v1/auth/login/admin", ownerEmail, ownerPassword)
	return a
}

func registerBuyer(t *testing.T, email string) string {
	t.Helper()
	if r := call(t, "POST", "/api/v1/auth/register", "", map[string]any{"name": "Pembeli", "email": email, "password": buyerPassword}); r.Status != 201 {
		t.Fatalf("register %s: %d %s", email, r.Status, r.Raw)
	}
	a, _ := login(t, "/api/v1/auth/login", email, buyerPassword)
	return a
}

// ------------------------------------------------------------------ DOKU palsu

// fakeTx: satu halaman Checkout di DOKU palsu, dikunci invoice (= reference_id kita).
type fakeTx struct {
	invoice, token, requestID string
	status                    string // transaction.status: PENDING, SUCCESS, EXPIRED, FAILED, CANCELLED
	service, channel          string // kosong = pembeli belum memilih channel
	amount                    int64
}

type fakeDoku struct {
	mu      sync.Mutex
	n       int
	txs     map[string]*fakeTx
	creates int
	lastReq doku.PaymentRequest
}

func newFakeDoku() *fakeDoku { return &fakeDoku{txs: map[string]*fakeTx{}} }

func (f *fakeDoku) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.txs, f.creates = map[string]*fakeTx{}, 0
}

func (f *fakeDoku) CreatePayment(_ context.Context, req doku.PaymentRequest) (*doku.PaymentResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	f.creates++
	f.lastReq = req
	tok, reqID := fmt.Sprintf("tok-%d", f.n), fmt.Sprintf("req-%d", f.n)
	f.txs[req.Order.InvoiceNumber] = &fakeTx{invoice: req.Order.InvoiceNumber, token: tok, requestID: reqID,
		status: doku.StatusPending, amount: req.Order.Amount}
	r := &doku.PaymentResponse{URL: "https://pay.test/checkout/" + tok, TokenID: tok, RequestID: reqID}
	r.Raw, _ = json.Marshal(map[string]any{"response": map[string]any{"payment": map[string]any{"url": r.URL, "token_id": tok}}})
	return r, nil
}

func (t *fakeTx) state() *doku.Status {
	s := &doku.Status{Order: doku.OrderInfo{InvoiceNumber: t.invoice, Amount: doku.Amount(fmt.Sprint(t.amount)), Status: doku.OrderGenerated},
		Transaction: doku.TransactionInfo{Status: t.status, OriginalRequestID: t.requestID},
		Service:     doku.Ref{ID: t.service}, Channel: doku.Ref{ID: t.channel}}
	if t.status == doku.StatusExpired {
		s.Order.Status = doku.OrderExpired
	}
	return s
}

func (f *fakeDoku) GetStatus(_ context.Context, invoice string) (*doku.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.txs[invoice]
	if t == nil {
		return nil, doku.ErrTransactionNotFound
	}
	return t.state(), nil
}

// Cancel seperti DOKU: hanya VA & QRIS yang masih PENDING; selain itu ditolak (400).
func (f *fakeDoku) Cancel(_ context.Context, invoice, originalRequestID, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.txs[invoice]
	switch {
	case t == nil:
		return &doku.APIError{HTTPStatus: 404, Messages: []string{"Not Found"}}
	case originalRequestID != t.requestID:
		return &doku.APIError{HTTPStatus: 400, Messages: []string{"payment.original_request_id is invalid"}}
	case t.status != doku.StatusPending:
		return &doku.APIError{HTTPStatus: 400, Messages: []string{"Transaction cannot be cancelled"}}
	case t.service != "VIRTUAL_ACCOUNT" && t.service != "QRIS":
		return &doku.APIError{HTTPStatus: 400, Messages: []string{"Cancellation is not supported for this channel"}}
	}
	t.status = "CANCELLED"
	return nil
}

// choose: pembeli memilih channel di halaman Checkout (transaksi tetap PENDING).
func (f *fakeDoku) choose(invoice, service, channel string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.txs[invoice]
	t.service, t.channel = service, channel
}

// complete: transaksi lunas (amount 0 = sesuai invoice). Halaman Checkout yang
// sudah ditutup di sisi toko (e-wallet/kartu) tetap bisa dibayar sampai batas waktunya.
func (f *fakeDoku) complete(invoice string, amount int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.txs[invoice]
	t.status = doku.StatusSuccess
	if t.channel == "" {
		t.service, t.channel = "VIRTUAL_ACCOUNT", "VIRTUAL_ACCOUNT_BRI"
	}
	if amount > 0 {
		t.amount = amount
	}
}

func (f *fakeDoku) setStatus(invoice, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.txs[invoice].status = status
}

func (f *fakeDoku) status(invoice string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.txs[invoice]; ok {
		return t.status
	}
	return ""
}

var notifySeq int

// notifyBody mengirim body notifikasi apa adanya, ditandatangani seperti DOKU
// (Request-Target = path webhook kita) dengan secret.
func notifyBody(t *testing.T, body []byte, secret string) resp {
	t.Helper()
	notifySeq++
	path := "/api/v1/webhooks/doku"
	reqID, ts := fmt.Sprintf("notif-%d", notifySeq), time.Now().UTC().Format(doku.TimestampLayout)
	return call(t, "POST", path, "", body, "Client-Id", dokuClientID, "Request-Id", reqID, "Request-Timestamp", ts,
		"Signature", doku.Sign(dokuClientID, reqID, ts, path, body, secret))
}

// notify mengirim notifikasi DOKU untuk invoice dengan status yang DIKLAIM (boleh
// beda dengan status sebenarnya di DOKU palsu).
func notify(t *testing.T, invoice, status, secret string) resp {
	t.Helper()
	fake.mu.Lock()
	tx := *fake.txs[invoice]
	fake.mu.Unlock()
	body := j(map[string]any{
		"order":       map[string]any{"invoice_number": invoice, "amount": tx.amount},
		"transaction": map[string]any{"status": status, "date": time.Now().UTC().Format(doku.TimestampLayout), "original_request_id": tx.requestID},
		"service":     map[string]any{"id": tx.service}, "acquirer": map[string]any{"id": "BRI"}, "channel": map[string]any{"id": tx.channel},
	})
	return notifyBody(t, []byte(body), secret)
}

// ------------------------------------------------------------------ Google palsu

func fakeGoogle() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			r.ParseForm()
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","expires_in":3600}`, r.Form.Get("code"))
		case "/userinfo":
			info := map[string]map[string]any{
				"good":       {"sub": "g-123", "email": "Budi@Gmail.com", "email_verified": true, "name": "Budi", "picture": "https://x/p.jpg"},
				"unverified": {"sub": "g-999", "email": "palsu@gmail.com", "email_verified": false, "name": "Palsu"},
				"local":      {"sub": "g-555", "email": "pembeli@test.local", "email_verified": true, "name": "Ambil Alih"},
			}[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
			json.NewEncoder(w).Encode(info)
		}
	}))
}
