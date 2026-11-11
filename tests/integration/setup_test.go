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
	paymentservices "backend/src/modules/payment/services"
	"backend/src/modules/payment/xendit"
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
	webhookToken   = "tok-rahasia"
	ownerEmail     = "owner@test.local"
	ownerPassword  = "rahasia-panjang-123"
	buyerPassword  = "rahasia-123"
)

var (
	srv      *httptest.Server
	specDoc  *openapi3.T
	oaRouter routers.Router
	fake     = newFakeXendit()
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

	paymentservices.Configure(fake, paymentservices.Settings{WebhookToken: webhookToken,
		SuccessURL: frontendOrigin + "/bayar/sukses", CancelURL: frontendOrigin + "/bayar/batal"})

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

// ------------------------------------------------------------------ Xendit palsu

type fakeXendit struct {
	mu       sync.Mutex
	n        int
	sessions map[string]*xendit.Session
	creates  int
}

func newFakeXendit() *fakeXendit { return &fakeXendit{sessions: map[string]*xendit.Session{}} }

func (f *fakeXendit) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions, f.creates = map[string]*xendit.Session{}, 0
}

func (f *fakeXendit) CreateSession(_ context.Context, req xendit.CreateSessionRequest) (*xendit.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	f.creates++
	id := fmt.Sprintf("ps-%d", f.n)
	s := &xendit.Session{PaymentSessionID: id, ReferenceID: req.ReferenceID, Status: "ACTIVE",
		PaymentLinkURL: "https://pay.test/" + id, ExpiresAt: req.ExpiresAt,
		Amount: json.Number(fmt.Sprint(req.Amount)), Currency: req.Currency}
	s.Raw, _ = json.Marshal(s)
	f.sessions[id] = s
	cp := *s
	return &cp, nil
}

func (f *fakeXendit) GetSession(_ context.Context, id string) (*xendit.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return nil, &xendit.APIError{HTTPStatus: 404, ErrorCode: "NOT_FOUND", Message: "session not found"}
	}
	cp := *s
	return &cp, nil
}

func (f *fakeXendit) CancelSession(_ context.Context, id string) (*xendit.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return nil, &xendit.APIError{HTTPStatus: 404, ErrorCode: "NOT_FOUND", Message: "session not found"}
	}
	if s.Status == "ACTIVE" {
		s.Status = "CANCELED"
	}
	cp := *s
	return &cp, nil
}

// complete menandai sesi lunas di Xendit palsu (amount 0 = sesuai sesi).
func (f *fakeXendit) complete(id string, amount int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.sessions[id]
	s.Status, s.PaymentID, s.PaymentRequestID = "COMPLETED", "py-"+id, "pr-"+id
	if amount > 0 {
		s.Amount = json.Number(fmt.Sprint(amount))
	}
}

func (f *fakeXendit) status(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.sessions[id]; ok {
		return s.Status
	}
	return ""
}

func webhook(t *testing.T, event, sessionID, token string) resp {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"event":%q,"business_id":"biz","created":%q,"data":{"payment_session_id":%q,"status":"X"}}`,
		event, time.Now().UTC().Format(time.RFC3339), sessionID))
	return call(t, "POST", "/api/v1/webhooks/xendit", "", body, "x-callback-token", token)
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
