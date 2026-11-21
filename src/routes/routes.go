package routes

import (
	httperror "backend/src/middlewares/Error"
	security "backend/src/middlewares/Security"
	authRoutes "backend/src/modules/auth/routes"
	cartRoutes "backend/src/modules/cart/routes"
	orderRoutes "backend/src/modules/order/routes"
	paymentRoutes "backend/src/modules/payment/routes"
	productsRoutes "backend/src/modules/products/routes"
	userRoutes "backend/src/modules/user/routes"
	"backend/src/utils/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)


type Options struct {
	CORSOrigins    []string // origin frontend yang boleh memanggil API dari browser
	TrustedProxies []string // proxy yang boleh menentukan IP klien (X-Forwarded-For)
	Docs           bool     // pasang Swagger UI di /docs
}


func NewEngine(opts Options) (*gin.Engine, error) {
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.HandleMethodNotAllowed = true
	if err := r.SetTrustedProxies(opts.TrustedProxies); err != nil {
		return nil, err
	}
	r.Use(
		security.RequestID(),
		security.AccessLog("/healthz", "/readyz"),
		security.Recovery(),
		security.Headers(),
		security.CORS(opts.CORSOrigins),
		security.BodyLimit(security.MaxBodyBytes),
	)
	r.GET("/healthz", healthz)
	r.GET("/readyz", readyz)
	if opts.Docs {
		registerDocs(r)
	}
	InitRoutes(r)
	r.NoRoute(func(c *gin.Context) { httperror.NotFoundError(c, "Endpoint tidak ditemukan") })
	r.NoMethod(func(c *gin.Context) { httperror.MethodNotAllowedError(c, "Method tidak diizinkan untuk endpoint ini") })
	return r, nil
}


func InitRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	userRoutes.RegisterUserRoutes(api)
	authRoutes.RegisterAuthRoutes(api)
	productsRoutes.RegisterProductsRoutes(api)
	cartRoutes.RegisterCartRoutes(api)
	orderRoutes.RegisterOrderRoutes(api)
	paymentRoutes.RegisterPaymentRoutes(api)
	api.GET("", func(c *gin.Context) {
		response.OK(c, "Welcome to Station Parfume API!", nil)
	})
}

func Handler(e *gin.Engine) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := r.URL.Path; len(p) > 1 && strings.HasSuffix(p, "/") {
			r.URL.Path = strings.TrimRight(p, "/")
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
			if r.URL.RawPath != "" {
				r.URL.RawPath = strings.TrimRight(r.URL.RawPath, "/")
			}
		}
		e.ServeHTTP(w, r)
	})
}
