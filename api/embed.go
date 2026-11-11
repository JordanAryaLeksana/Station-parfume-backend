// Package api berisi spesifikasi OpenAPI 3 (Swagger) Station Parfume API.
// File openapi.yaml di-embed ke binary dan disajikan di /docs/openapi.yaml;
// Swagger UI di /docs. Test memastikan spec selalu cocok dengan rute dan response.
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte
