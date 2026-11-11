// Package security berisi middleware HTTP global: request ID, log akses,
// recovery, header keamanan, CORS, dan batas ukuran body.
package security

import (
	httperror "backend/src/middlewares/Error"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// MaxBodyBytes: batas body request (JSON). Upload file tidak lewat API ini
	// (foto = link CDN), jadi 1 MiB sangat longgar.
	MaxBodyBytes = 1 << 20

	RequestIDHeader = "X-Request-ID"
	// CtxRequestID: key gin.Context berisi request ID.
	CtxRequestID = "requestID"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID memakai X-Request-ID dari klien kalau formatnya aman, kalau tidak
// membuat UUID baru. ID dikembalikan di header response dan dicatat di log.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !requestIDPattern.MatchString(id) {
			id = uuid.NewString()
		}
		c.Set(CtxRequestID, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// AccessLog mencatat satu baris per request TANPA query string — query bisa
// berisi kode OAuth (code, state) atau data pribadi. Path di skip tidak dicatat.
func AccessLog(skip ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		if slices.Contains(skip, path) {
			return
		}
		log.Printf("%s %s %d %s ip=%s rid=%s", c.Request.Method, path, c.Writer.Status(),
			time.Since(start).Round(time.Microsecond), c.ClientIP(), c.GetString(CtxRequestID))
	}
}

// Recovery: panic → 500 JSON. Stack trace + request (header Authorization
// disamarkan oleh Gin) hanya ke log.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, _ any) {
		httperror.InternalServerError(c, "Internal server error")
	})
}

// Headers memasang header keamanan di semua response. API hanya mengirim
// JSON, jadi CSP paling ketat; halaman /docs menimpa CSP-nya sendiri.
func Headers() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Cache-Control", "no-store")
		// HSTS hanya berarti lewat HTTPS (TLS langsung atau di belakang proxy TLS).
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		c.Next()
	}
}

// CORS mengizinkan origin frontend yang terdaftar. Request dari origin lain
// (bukan origin API sendiri) dijawab 403 JSON. Daftar kosong = hanya
// same-origin (mis. Swagger UI di /docs) dan klien non-browser.
func CORS(origins []string) gin.HandlerFunc {
	var handler gin.HandlerFunc
	if len(origins) > 0 {
		handler = cors.New(cors.Config{
			AllowOrigins:     origins,
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
			AllowHeaders:     []string{"Authorization", "Content-Type", RequestIDHeader},
			ExposeHeaders:    []string{"Retry-After", RequestIDHeader},
			AllowCredentials: false, // token lewat header Authorization, bukan cookie
			MaxAge:           12 * time.Hour,
		})
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" || origin == "http://"+c.Request.Host || origin == "https://"+c.Request.Host {
			c.Next() // bukan request lintas origin
			return
		}
		if !slices.Contains(origins, strings.ToLower(origin)) {
			httperror.ForbiddenError(c, "Origin tidak diizinkan (CORS): tambahkan ke CORS_ALLOWED_ORIGINS")
			return
		}
		handler(c)
	}
}

// BodyLimit menolak body lebih dari n byte: 413 kalau Content-Length sudah
// kelewat, dan memotong body chunked lewat http.MaxBytesReader.
func BodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > n {
			httperror.PayloadTooLargeError(c, "Body request terlalu besar")
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		}
		c.Next()
	}
}
