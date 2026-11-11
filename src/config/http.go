package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ApplyGinMode memakai GIN_MODE dari env. Gin membaca GIN_MODE di init()
// package-nya — sebelum .env dimuat — jadi nilai di .env harus diterapkan ulang
// setelah godotenv.Load(). Kosong = debug.
func ApplyGinMode() error {
	mode := strings.TrimSpace(os.Getenv("GIN_MODE"))
	switch mode {
	case "":
		mode = gin.DebugMode
	case gin.DebugMode, gin.ReleaseMode, gin.TestMode:
	default:
		return fmt.Errorf("GIN_MODE %q tidak dikenal (pilih: debug, release, test)", mode)
	}
	gin.SetMode(mode)
	return nil
}

// CORSOrigins: origin frontend yang boleh memanggil API dari browser.
// Dari CORS_ALLOWED_ORIGINS (dipisah koma); kalau kosong, origin dari
// FRONTEND_URL. Kosong semua = tidak ada origin lain yang diizinkan.
// Setiap origin harus berbentuk scheme://host[:port] — "*" ditolak supaya
// API tidak terbuka untuk semua situs.
func CORSOrigins() ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	fromFrontend := false
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("FRONTEND_URL"))
		fromFrontend = true
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		origin, err := parseOrigin(part, fromFrontend)
		if err != nil {
			return nil, err
		}
		out = append(out, origin)
	}
	return out, nil
}

func parseOrigin(s string, allowPath bool) (string, error) {
	if s == "*" {
		return "", fmt.Errorf("CORS_ALLOWED_ORIGINS tidak boleh \"*\": sebutkan domain frontend satu per satu")
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("origin CORS %q tidak valid: harus seperti https://shop.example.com", s)
	}
	// FRONTEND_URL boleh punya path (mis. https://shop.test/app); origin-nya saja yang dipakai.
	if !allowPath && ((u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "") {
		return "", fmt.Errorf("origin CORS %q tidak boleh memuat path/query: cukup scheme://host[:port]", s)
	}
	if u.User != nil {
		return "", fmt.Errorf("origin CORS %q tidak boleh memuat user/password", s)
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}

// DocsEnabled: Swagger UI (/docs) aktif? API_DOCS_ENABLED=true/false menang;
// kalau kosong, aktif di luar mode release.
func DocsEnabled() (bool, error) {
	v := strings.TrimSpace(os.Getenv("API_DOCS_ENABLED"))
	if v == "" {
		return gin.Mode() != gin.ReleaseMode, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("API_DOCS_ENABLED harus true/false")
	}
	return b, nil
}
