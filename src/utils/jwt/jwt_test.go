package jwt

import (
	"backend/src/repository"
	"errors"
	"strings"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

var (
	accessSecret  = strings.Repeat("a", 32)
	refreshSecret = strings.Repeat("r", 32)
)

func setup(t *testing.T) repository.User {
	t.Setenv("JWT_SECRET", accessSecret)
	t.Setenv("JWT_REFRESH_SECRET", refreshSecret)
	return repository.User{ID: 7, Email: "a@b.c", Role: "owner"}
}

func TestRoundTrip(t *testing.T) {
	u := setup(t)
	acc, err := GenerateAccessToken(u)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseAccessToken(acc, accessSecret)
	if err != nil || c.UserID != 7 || c.Role != "owner" || c.ID == "" || c.Issuer != Issuer {
		t.Fatalf("access: %+v %v", c, err)
	}
	ref, _ := GenerateRefreshToken(u)
	if _, err := ParseRefreshToken(ref, refreshSecret); err != nil {
		t.Fatalf("refresh: %v", err)
	}
}

func forge(t *testing.T, method gojwt.SigningMethod, key any, mutate func(*JwtCustomClaims)) string {
	t.Helper()
	c := JwtCustomClaims{UserID: 1, Role: "owner", TokenType: TypeAccess, RegisteredClaims: gojwt.RegisteredClaims{
		Issuer: Issuer, ID: "jti-1", ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour))}}
	if mutate != nil {
		mutate(&c)
	}
	s, err := gojwt.NewWithClaims(method, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRejects(t *testing.T) {
	u := setup(t)
	ref, _ := GenerateRefreshToken(u)
	acc, _ := GenerateAccessToken(u)
	cases := map[string]func() error{
		"refresh dipakai sebagai access": func() error { _, err := ParseAccessToken(ref, accessSecret); return err },
		"access dipakai sebagai refresh": func() error { _, err := ParseRefreshToken(acc, refreshSecret); return err },
		"secret salah":                   func() error { _, err := ParseAccessToken(acc, refreshSecret); return err },
		"secret kosong": func() error {
			_, err := ParseAccessToken(forge(t, gojwt.SigningMethodHS256, []byte(""), nil), "")
			return err
		},
		"alg HS512": func() error {
			_, err := ParseAccessToken(forge(t, gojwt.SigningMethodHS512, []byte(accessSecret), nil), accessSecret)
			return err
		},
		"alg none": func() error {
			_, err := ParseAccessToken(forge(t, gojwt.SigningMethodNone, gojwt.UnsafeAllowNoneSignatureType, nil), accessSecret)
			return err
		},
		"issuer lain": func() error {
			_, err := ParseAccessToken(forge(t, gojwt.SigningMethodHS256, []byte(accessSecret), func(c *JwtCustomClaims) { c.Issuer = "lain" }), accessSecret)
			return err
		},
		"tanpa exp": func() error {
			_, err := ParseAccessToken(forge(t, gojwt.SigningMethodHS256, []byte(accessSecret), func(c *JwtCustomClaims) { c.ExpiresAt = nil }), accessSecret)
			return err
		},
		"kedaluwarsa": func() error {
			_, err := ParseAccessToken(forge(t, gojwt.SigningMethodHS256, []byte(accessSecret), func(c *JwtCustomClaims) {
				c.ExpiresAt = gojwt.NewNumericDate(time.Now().Add(-time.Minute))
			}), accessSecret)
			return err
		},
		"tanpa jti": func() error {
			_, err := ParseAccessToken(forge(t, gojwt.SigningMethodHS256, []byte(accessSecret), func(c *JwtCustomClaims) { c.ID = "" }), accessSecret)
			return err
		},
	}
	for name, f := range cases {
		if err := f(); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: harus ErrInvalidToken, got %v", name, err)
		}
	}
	t.Setenv("JWT_SECRET", "")
	if _, err := GenerateAccessToken(u); err == nil {
		t.Error("membuat token dengan secret kosong harus error")
	}
}
