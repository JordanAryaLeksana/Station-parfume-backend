package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSOrigins(t *testing.T) {
	cases := []struct {
		name, origins, frontend string
		want                    []string
		wantErr                 string
	}{
		{"daftar", " https://Shop.test , http://localhost:3000 ", "", []string{"https://shop.test", "http://localhost:3000"}, ""},
		{"fallback FRONTEND_URL (path dibuang)", "", "https://shop.test/app", []string{"https://shop.test"}, ""},
		{"kosong", "", "", nil, ""},
		{"bintang ditolak", "*", "", nil, "tidak boleh"},
		{"path ditolak", "https://shop.test/app", "", nil, "path"},
		{"bukan http", "ftp://shop.test", "", nil, "tidak valid"},
		{"tanpa scheme", "shop.test", "", nil, "tidak valid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CORS_ALLOWED_ORIGINS", tc.origins)
			t.Setenv("FRONTEND_URL", tc.frontend)
			got, err := CORSOrigins()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, mau mengandung %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("= %v, %v; mau %v", got, err, tc.want)
			}
		})
	}
}

func TestDocsEnabledAndGinMode(t *testing.T) {
	defer gin.SetMode(gin.TestMode)
	t.Setenv("GIN_MODE", "release")
	t.Setenv("API_DOCS_ENABLED", "")
	if err := ApplyGinMode(); err != nil || gin.Mode() != gin.ReleaseMode {
		t.Fatalf("GIN_MODE release: %v %s", err, gin.Mode())
	}
	if on, _ := DocsEnabled(); on {
		t.Fatal("docs harus mati default di release")
	}
	t.Setenv("API_DOCS_ENABLED", "true")
	if on, _ := DocsEnabled(); !on {
		t.Fatal("API_DOCS_ENABLED=true harus menyalakan docs")
	}
	t.Setenv("API_DOCS_ENABLED", "mungkin")
	if _, err := DocsEnabled(); err == nil {
		t.Fatal("nilai tidak valid harus error")
	}
	t.Setenv("GIN_MODE", "")
	t.Setenv("API_DOCS_ENABLED", "")
	if err := ApplyGinMode(); err != nil || gin.Mode() != gin.DebugMode {
		t.Fatalf("GIN_MODE kosong → debug: %v %s", err, gin.Mode())
	}
	if on, _ := DocsEnabled(); !on {
		t.Fatal("docs aktif default di debug")
	}
	t.Setenv("GIN_MODE", "produksi")
	if err := ApplyGinMode(); err == nil {
		t.Fatal("GIN_MODE tidak dikenal harus error")
	}
}
