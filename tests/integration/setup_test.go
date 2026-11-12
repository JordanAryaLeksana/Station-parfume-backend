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
	"backend/src/modules/payment/midtrans"
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
	serverKey      = "SB-Mid-server-test"
	ownerEmail     = "owner@test.local"
	ownerPassword  = "rahasia-panjang-123"
	buyerPassword  = "rahasia-123"
)

var (
	srv      *httptest.Server
	specDoc  *openapi3.T
	oaRouter routers.Router
	fake     = newFakeMidtrans()
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

	paymentservices.Configure(fake, paymentservices.Settings{ServerKey: serverKey, SuccessURL: frontendOrigin + "/bayar/sukses"})

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

// ------------------------------------------------------------------ Midtrans palsu

// fakeTx: satu transaksi Snap di Midtrans palsu, dikunci order_id (= reference_id kita).
type fakeTx struct {
	orderID, token, txID string
	status               string // "" = pembeli belum memilih metode bayar (status → 404)
	amount               int64
}

type fakeMidtrans struct {
	mu      sync.Mutex
	n       int
	txs     map[string]*fakeTx
	creates int
}

func newFakeMidtrans() *fakeMidtrans { return &fakeMidtrans{txs: map[string]*fakeTx{}} }

func (f *fakeMidtrans) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.txs, f.creates = map[string]*fakeTx{}, 0
}

func (f *fakeMidtrans) CreateSnap(_ context.Context, req midtrans.SnapRequest) (*midtrans.SnapResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	f.creates++
	tok := fmt.Sprintf("snap-%d", f.n)
	f.txs[req.TransactionDetails.OrderID] = &fakeTx{orderID: req.TransactionDetails.OrderID, token: tok, amount: req.TransactionDetails.GrossAmount}
	r := &midtrans.SnapResponse{Token: tok, RedirectURL: "https://pay.test/snap/" + tok}
	r.Raw, _ = json.Marshal(r)
	return r, nil
}

// find: id = order_id atau transaction_id (seperti API Midtrans).
func (f *fakeMidtrans) find(id string) *fakeTx {
	if t, ok := f.txs[id]; ok {
		return t
	}
	for _, t := range f.txs {
		if t.txID != "" && t.txID == id {
			return t
		}
	}
	return nil
}

func fakeStatusCode(status string) string {
	switch status {
	case midtrans.StatusPending:
		return "201"
	case midtrans.StatusExpire:
		return "407"
	case midtrans.StatusDeny:
		return "202"
	}
	return "200"
}

func (t *fakeTx) transaction() *midtrans.Transaction {
	return &midtrans.Transaction{StatusCode: fakeStatusCode(t.status), TransactionID: t.txID, OrderID: t.orderID,
		GrossAmount: fmt.Sprintf("%d.00", t.amount), Currency: "IDR", PaymentType: "bank_transfer",
		TransactionStatus: t.status, FraudStatus: midtrans.FraudAccept, VANumbers: []midtrans.VANumber{{Bank: "bca", VANumber: "12345"}}}
}

func (f *fakeMidtrans) GetStatus(_ context.Context, id string) (*midtrans.Transaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.find(id)
	if t == nil || t.status == "" {
		return nil, midtrans.ErrTransactionNotFound
	}
	return t.transaction(), nil
}

func (f *fakeMidtrans) Expire(_ context.Context, id string) (*midtrans.Transaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.find(id)
	if t == nil || t.status == "" {
		return nil, midtrans.ErrTransactionNotFound
	}
	if t.status != midtrans.StatusPending {
		return nil, &midtrans.APIError{HTTPStatus: 200, StatusCode: "412", Messages: []string{"Merchant cannot modify the status of the transaction"}}
	}
	t.status = midtrans.StatusExpire
	return t.transaction(), nil
}

// choose: pembeli memilih metode bayar di halaman Snap → transaksi pending.
func (f *fakeMidtrans) choose(orderID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.txs[orderID]
	t.status, t.txID = midtrans.StatusPending, "tx-"+t.token
}

// complete: transaksi lunas (amount 0 = sesuai transaksi). order_id yang sudah
// di-expire juga bisa dibayar lagi lewat halaman Snap yang masih terbuka.
func (f *fakeMidtrans) complete(orderID string, amount int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.txs[orderID]
	t.status, t.txID = midtrans.StatusSettlement, "tx-"+t.token
	if amount > 0 {
		t.amount = amount
	}
}

func (f *fakeMidtrans) status(orderID string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.txs[orderID]; ok {
		return t.status
	}
	return ""
}

// notify mengirim notifikasi Midtrans untuk orderID dengan status yang DIKLAIM
// (boleh beda dengan status sebenarnya di Midtrans palsu), ditandatangani dengan key.
func notify(t *testing.T, orderID, status, key string) resp {
	t.Helper()
	fake.mu.Lock()
	tx := *fake.txs[orderID]
	fake.mu.Unlock()
	code, gross := fakeStatusCode(status), fmt.Sprintf("%d.00", tx.amount)
	body := j(map[string]any{
		"transaction_time": time.Now().Format("2006-01-02 15:04:05"), "transaction_status": status, "transaction_id": tx.txID,
		"status_message": "midtrans payment notification", "status_code": code, "signature_key": midtrans.Signature(orderID, code, gross, key),
		"payment_type": "bank_transfer", "order_id": orderID, "gross_amount": gross, "fraud_status": "accept", "currency": "IDR",
		"va_numbers": []map[string]string{{"bank": "bca", "va_number": "12345"}},
	})
	return call(t, "POST", "/api/v1/webhooks/midtrans", "", []byte(body))
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
