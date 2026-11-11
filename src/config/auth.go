package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// MinJWTSecretLen: secret HS256 minimal 32 karakter (256 bit).
const MinJWTSecretLen = 32

// ValidateJWTSecrets menolak secret kosong/pendek atau access = refresh.
// Tanpa ini, JWT_SECRET kosong membuat siapa pun bisa memalsukan token.
func ValidateJWTSecrets(access, refresh string) error {
	var errs []error
	if len(access) < MinJWTSecretLen {
		errs = append(errs, fmt.Errorf("JWT_SECRET wajib diisi, minimal %d karakter", MinJWTSecretLen))
	}
	if len(refresh) < MinJWTSecretLen {
		errs = append(errs, fmt.Errorf("JWT_REFRESH_SECRET wajib diisi, minimal %d karakter", MinJWTSecretLen))
	}
	if access != "" && access == refresh {
		errs = append(errs, errors.New("JWT_SECRET dan JWT_REFRESH_SECRET harus berbeda"))
	}
	return errors.Join(errs...)
}

// LoadAuthConfig dipanggil di awal main; error = server tidak boleh start.
func LoadAuthConfig() error {
	return ValidateJWTSecrets(os.Getenv("JWT_SECRET"), os.Getenv("JWT_REFRESH_SECRET"))
}

// TrustedProxies dari env TRUSTED_PROXIES (dipisah koma, mis. "127.0.0.1,10.0.0.0/8").
// Kosong = tidak ada proxy yang dipercaya: ClientIP() = alamat koneksi langsung,
// header X-Forwarded-For diabaikan (tidak bisa dipalsukan untuk lolos rate limit).
func TrustedProxies() []string {
	var out []string
	for _, p := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
