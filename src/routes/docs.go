package routes

import (
	"backend/api"
	"crypto/sha256"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Swagger UI dimuat dari jsDelivr dengan versi tetap.
const swaggerUIBase = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.33.1"

const swaggerInit = `window.onload=function(){window.ui=SwaggerUIBundle({url:"/docs/openapi.yaml",dom_id:"#swagger-ui",deepLinking:true,validatorUrl:null,persistAuthorization:false});};`

var (
	docsHTML = []byte(`<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Station Parfume API</title>
<link rel="stylesheet" href="` + swaggerUIBase + `/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="` + swaggerUIBase + `/swagger-ui-bundle.js"></script>
<script>` + swaggerInit + `</script>
</body>
</html>
`)
	// CSP halaman docs: hanya CDN swagger-ui + script inisialisasi (hash),
	// request API hanya ke origin sendiri.
	docsCSP = func() string {
		sum := sha256.Sum256([]byte(swaggerInit))
		hash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
		return "default-src 'none'; script-src https://cdn.jsdelivr.net " + hash +
			"; style-src https://cdn.jsdelivr.net 'unsafe-inline'; img-src 'self' data: https://cdn.jsdelivr.net" +
			"; font-src data: https://cdn.jsdelivr.net; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	}()
)

// registerDocs: GET /docs (Swagger UI) dan GET /docs/openapi.yaml (spec).
func registerDocs(r *gin.Engine) {
	r.GET("/docs", func(c *gin.Context) {
		c.Header("Content-Security-Policy", docsCSP)
		c.Data(http.StatusOK, "text/html; charset=utf-8", docsHTML)
	})
	r.GET("/docs/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", api.OpenAPI)
	})
}
