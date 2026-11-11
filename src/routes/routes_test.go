package routes

import (
	"backend/api"
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newTestEngine(t *testing.T, opts Options) http.Handler {
	t.Helper()
	e, err := NewEngine(opts)
	if err != nil {
		t.Fatal(err)
	}
	e.GET("/test/panic", func(*gin.Context) { panic("boom") })
	return Handler(e)
}

func do(h http.Handler, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Host = "api.test"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body bukan JSON (%d): %q", w.Code, w.Body.String())
	}
	for _, k := range []string{"error", "message", "status"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("body error tanpa %q: %v", k, m)
		}
	}
	return m
}

// Spec OpenAPI harus valid dan memuat tepat semua rute Gin (tidak lebih, tidak kurang).
func TestOpenAPISpecMatchesRoutes(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData(api.OpenAPI)
	if err != nil {
		t.Fatalf("spec tidak bisa dibaca: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("spec tidak valid: %v", err)
	}
	inSpec := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			inSpec[strings.ToUpper(method)+" "+path] = true
		}
	}
	e, err := NewEngine(Options{Docs: true})
	if err != nil {
		t.Fatal(err)
	}
	inGin := map[string]bool{}
	for _, r := range e.Routes() {
		if strings.HasPrefix(r.Path, "/docs") {
			continue // halaman dokumentasi itu sendiri
		}
		p := r.Path
		for _, seg := range strings.Split(p, "/") {
			if strings.HasPrefix(seg, ":") {
				p = strings.Replace(p, seg, "{"+seg[1:]+"}", 1)
			}
		}
		inGin[r.Method+" "+p] = true
	}
	var missing, extra []string
	for k := range inGin {
		if !inSpec[k] {
			missing = append(missing, k)
		}
	}
	for k := range inSpec {
		if !inGin[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("rute belum didokumentasikan di api/openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(extra) > 0 {
		t.Errorf("api/openapi.yaml memuat rute yang tidak ada:\n  %s", strings.Join(extra, "\n  "))
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := newTestEngine(t, Options{})
	w := do(h, "GET", "/healthz", nil, nil)
	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"Cache-Control":           "no-store",
	}
	for k, v := range want {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, mau %q", k, got, v)
		}
	}
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS tidak boleh dikirim lewat HTTP biasa")
	}
	w = do(h, "GET", "/healthz", nil, map[string]string{"X-Forwarded-Proto": "https"})
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Error("HSTS harus ada lewat HTTPS")
	}
}

func TestCORS(t *testing.T) {
	h := newTestEngine(t, Options{CORSOrigins: []string{"https://shop.test"}})
	// preflight dari origin terdaftar
	w := do(h, "OPTIONS", "/api/v1/cart", nil, map[string]string{
		"Origin": "https://shop.test", "Access-Control-Request-Method": "GET",
		"Access-Control-Request-Headers": "authorization",
	})
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "https://shop.test" ||
		!strings.Contains(strings.ToLower(w.Header().Get("Access-Control-Allow-Headers")), "authorization") {
		t.Fatalf("preflight: %d %v", w.Code, w.Header())
	}
	// request biasa dari origin terdaftar
	w = do(h, "GET", "/api/v1", nil, map[string]string{"Origin": "https://shop.test"})
	if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "https://shop.test" ||
		!strings.Contains(w.Header().Get("Access-Control-Expose-Headers"), "Retry-After") {
		t.Fatalf("GET lintas origin: %d %v", w.Code, w.Header())
	}
	// origin lain ditolak 403 JSON
	w = do(h, "GET", "/api/v1", nil, map[string]string{"Origin": "https://evil.test"})
	if w.Code != http.StatusForbidden || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("origin asing: %d %v", w.Code, w.Header())
	}
	errorBody(t, w)
	// same-origin (Swagger UI di /docs) dan klien tanpa Origin tetap jalan
	if w = do(h, "GET", "/api/v1", nil, map[string]string{"Origin": "http://api.test"}); w.Code != 200 {
		t.Fatalf("same-origin: %d", w.Code)
	}
	if w = do(h, "GET", "/api/v1", nil, nil); w.Code != 200 {
		t.Fatalf("tanpa Origin: %d", w.Code)
	}
	// tanpa daftar origin: semua origin asing ditolak
	h = newTestEngine(t, Options{})
	if w = do(h, "OPTIONS", "/api/v1/cart", nil, map[string]string{"Origin": "https://shop.test", "Access-Control-Request-Method": "GET"}); w.Code != http.StatusForbidden {
		t.Fatalf("tanpa CORS_ALLOWED_ORIGINS: %d", w.Code)
	}
}

func TestBodyLimit(t *testing.T) {
	h := newTestEngine(t, Options{})
	big := bytes.Repeat([]byte("a"), (1<<20)+1)
	w := do(h, "POST", "/api/v1/auth/login", big, map[string]string{"Content-Type": "application/json"})
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("body > 1 MiB: %d", w.Code)
	}
	errorBody(t, w)
}

func TestTrailingSlashWithoutRedirect(t *testing.T) {
	h := newTestEngine(t, Options{})
	for _, p := range []string{"/api/v1", "/api/v1/", "/healthz", "/healthz/", "/api/v1//"} {
		if w := do(h, "GET", p, nil, nil); w.Code != 200 {
			t.Errorf("GET %s → %d (Location %q)", p, w.Code, w.Header().Get("Location"))
		}
	}
}

func TestNotFoundAndMethodNotAllowedAreJSON(t *testing.T) {
	h := newTestEngine(t, Options{})
	w := do(h, "GET", "/api/v1/tidak-ada", nil, nil)
	if w.Code != 404 {
		t.Fatalf("404: %d", w.Code)
	}
	errorBody(t, w)
	w = do(h, "DELETE", "/api/v1/categories", nil, nil)
	if w.Code != 405 {
		t.Fatalf("405: %d", w.Code)
	}
	errorBody(t, w)
}

func TestRequestID(t *testing.T) {
	h := newTestEngine(t, Options{})
	w := do(h, "GET", "/healthz", nil, map[string]string{"X-Request-ID": "abc-12345678"})
	if w.Header().Get("X-Request-ID") != "abc-12345678" {
		t.Fatalf("request ID valid harus dipakai ulang: %q", w.Header().Get("X-Request-ID"))
	}
	w = do(h, "GET", "/healthz", nil, map[string]string{"X-Request-ID": "<script>"})
	if id := w.Header().Get("X-Request-ID"); id == "" || id == "<script>" {
		t.Fatalf("request ID tidak valid harus diganti: %q", id)
	}
}

func TestPanicIsJSON500AndLogHasNoQuery(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	gin.DefaultErrorWriter = &buf
	defer func() { gin.DefaultErrorWriter = os.Stderr }()

	h := newTestEngine(t, Options{})
	w := do(h, "GET", "/test/panic", nil, nil)
	if w.Code != 500 {
		t.Fatalf("panic: %d", w.Code)
	}
	errorBody(t, w)
	do(h, "GET", "/api/v1?code=RAHASIA-OAUTH&state=xyz", nil, nil)
	if strings.Contains(buf.String(), "RAHASIA-OAUTH") {
		t.Fatal("query string (kode OAuth) tercatat di log")
	}
	if !strings.Contains(buf.String(), "GET /api/v1 200") {
		t.Fatalf("log akses tidak ada: %q", buf.String())
	}
}

func TestReadyzWithoutDependencies(t *testing.T) {
	h := newTestEngine(t, Options{})
	w := do(h, "GET", "/readyz", nil, nil)
	var body healthBody
	json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 503 || body.Status != "unavailable" || body.Checks["database"] != "unavailable" || body.Checks["redis"] != "unavailable" {
		t.Fatalf("readyz tanpa DB/Redis: %d %+v", w.Code, body)
	}
	if w = do(h, "GET", "/healthz", nil, nil); w.Code != 200 {
		t.Fatalf("healthz: %d", w.Code)
	}
}

func TestDocs(t *testing.T) {
	h := newTestEngine(t, Options{Docs: true})
	w := do(h, "GET", "/docs", nil, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "swagger-ui-dist@5.33.1") {
		t.Fatalf("/docs: %d", w.Code)
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "'sha256-") || strings.Contains(csp, "script-src 'unsafe-inline'") {
		t.Fatalf("CSP docs: %q", csp)
	}
	w = do(h, "GET", "/docs/openapi.yaml", nil, nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), api.OpenAPI) {
		t.Fatalf("/docs/openapi.yaml: %d", w.Code)
	}
	h = newTestEngine(t, Options{Docs: false})
	if w = do(h, "GET", "/docs", nil, nil); w.Code != 404 {
		t.Fatalf("/docs harus mati kalau Docs=false: %d", w.Code)
	}
}
