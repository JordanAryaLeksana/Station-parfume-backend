//go:build integration

package integration

import (
	"backend/src/config"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
)

func flushRateLimit(t *testing.T) {
	ctx := context.Background()
	keys, _ := config.RedisClient.Keys(ctx, "rl:*").Result()
	if len(keys) > 0 {
		config.RedisClient.Del(ctx, keys...)
	}
}

func TestAuth(t *testing.T) {
	resetData(t)

	// ---------------- register
	r := call(t, "POST", "/api/v1/auth/register", "", map[string]any{"name": "Pembeli", "email": "pembeli@test.local"})
	check(t, "register tanpa password → 400", r.Status == 400, r.Status)
	r = call(t, "POST", "/api/v1/auth/register", "", map[string]any{"name": "Pembeli", "email": "pembeli@test.local", "password": "pendek"})
	check(t, "register password < 8 → 400", r.Status == 400, r.Status)
	r = call(t, "POST", "/api/v1/auth/register", "", map[string]any{"name": "Pembeli", "email": "pembeli@test.local", "password": buyerPassword})
	check(t, "register valid → 201 {message, data}", r.Status == 201 && r.Data()["email"] == "pembeli@test.local", r.Status, string(r.Raw))
	r = call(t, "POST", "/api/v1/auth/register", "", map[string]any{"name": "X", "email": "PEMBELI@test.local", "password": buyerPassword})
	check(t, "register email dobel (beda huruf besar) → 409", r.Status == 409, r.Status)

	// ---------------- login: pesan seragam
	r1 := call(t, "POST", "/api/v1/auth/login", "", map[string]any{"email": "tidakada@test.local", "password": buyerPassword})
	r2 := call(t, "POST", "/api/v1/auth/login", "", map[string]any{"email": "pembeli@test.local", "password": "salah-salah"})
	r3 := call(t, "POST", "/api/v1/auth/login/admin", "", map[string]any{"email": "pembeli@test.local", "password": buyerPassword})
	check(t, "email tidak ada / password salah / pembeli di login staf → 401 pesan sama",
		r1.Status == 401 && r2.Status == 401 && r3.Status == 401 && r1.Message() == r2.Message() && r2.Message() == r3.Message() &&
			r1.Message() == "Email atau password salah", r1.Status, r2.Status, r3.Status, r1.Message(), r3.Message())
	acc, ref := login(t, "/api/v1/auth/login", "pembeli@test.local", buyerPassword)

	// ---------------- logout
	r = call(t, "GET", "/api/v1/cart", acc, nil)
	check(t, "token aktif → GET /cart 200 (tanpa garis miring, tanpa redirect)", r.Status == 200, r.Status)
	r = call(t, "POST", "/api/v1/auth/logout", acc, map[string]any{"refresh_token": ref})
	check(t, "logout → 204 tanpa body", r.Status == 204 && len(r.Raw) == 0, r.Status)
	check(t, "access token setelah logout → 401", call(t, "GET", "/api/v1/cart", acc, nil).Status == 401)
	check(t, "refresh token yang dikirim saat logout ikut dicabut", call(t, "POST", "/api/v1/auth/refresh", ref, nil).Status == 401)

	// ---------------- refresh sekali pakai
	acc, ref = login(t, "/api/v1/auth/login", "pembeli@test.local", buyerPassword)
	r = call(t, "POST", "/api/v1/auth/refresh", ref, nil)
	acc2, _ := r.Data()["access_token"].(string)
	ref2, _ := r.Data()["refresh_token"].(string)
	check(t, "refresh → 200 + pasangan token baru", r.Status == 200 && acc2 != "" && ref2 != "" && ref2 != ref, r.Status)
	check(t, "refresh token lama dipakai ulang → 401", call(t, "POST", "/api/v1/auth/refresh", ref, nil).Status == 401)
	check(t, "access token di /auth/refresh → 401", call(t, "POST", "/api/v1/auth/refresh", acc2, nil).Status == 401)
	check(t, "refresh token sebagai access → 401", call(t, "GET", "/api/v1/cart", ref2, nil).Status == 401)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if call(t, "POST", "/api/v1/auth/refresh", ref2, nil).Status == 200 {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	check(t, "5 refresh bersamaan dengan token sama → tepat 1 berhasil", ok == 1, ok)

	// ---------------- staf: peran dibaca ulang dari DB
	owner := ownerToken(t)
	call(t, "POST", "/api/v1/users", owner, map[string]any{"name": "Gudang", "email": "gudang@test.local", "password": buyerPassword, "role": "warehouse"})
	call(t, "POST", "/api/v1/users", owner, map[string]any{"name": "Admin2", "email": "admin2@test.local", "password": buyerPassword, "role": "admin"})
	gudang, _ := login(t, "/api/v1/auth/login/admin", "gudang@test.local", buyerPassword)
	admin2, _ := login(t, "/api/v1/auth/login/admin", "admin2@test.local", buyerPassword)
	check(t, "gudang → riwayat stok 200", call(t, "GET", "/api/v1/admin/inventory/movements", gudang, nil).Status == 200)
	gid := scalar[uint](t, "select id from users where email = 'gudang@test.local'")
	r = call(t, "PUT", fmt.Sprintf("/api/v1/users/%d", gid), owner, map[string]any{"name": "Gudang", "email": "gudang@test.local", "password": buyerPassword, "role": "user"})
	check(t, "owner menurunkan gudang jadi pembeli", r.Status == 200, r.Status)
	check(t, "token lama gudang langsung 403", call(t, "GET", "/api/v1/admin/inventory/movements", gudang, nil).Status == 403)
	check(t, "admin2 → /admin/products 200", call(t, "GET", "/api/v1/admin/products", admin2, nil).Status == 200)
	a2 := scalar[uint](t, "select id from users where email = 'admin2@test.local'")
	r = call(t, "DELETE", fmt.Sprintf("/api/v1/users/%d", a2), owner, nil)
	check(t, "admin2 dihapus → token lamanya 401", r.Status == 200 && call(t, "GET", "/api/v1/admin/products", admin2, nil).Status == 401, r.Status)
	check(t, "/admins sudah tidak ada → 404", call(t, "POST", "/api/v1/admins", owner, map[string]any{}).Status == 404)

	// ---------------- rate limit
	flushRateLimit(t)
	var codes []int
	for i := 0; i < 10; i++ {
		codes = append(codes, call(t, "POST", "/api/v1/auth/login", "", map[string]any{"email": "pembeli@test.local", "password": "salah-salah"}).Status)
	}
	r = call(t, "POST", "/api/v1/auth/login", "", map[string]any{"email": "pembeli@test.local", "password": "salah-salah"})
	check(t, "login ke-11 dalam 1 menit → 429 + Retry-After", r.Status == 429 && r.Header.Get("Retry-After") != "" && codes[9] == 401, r.Status, codes)
	r = call(t, "POST", "/api/v1/auth/login", "", map[string]any{"email": "pembeli@test.local", "password": buyerPassword}, "X-Forwarded-For", "203.0.113.9")
	check(t, "X-Forwarded-For palsu tidak melewati rate limit", r.Status == 429, r.Status)
	flushRateLimit(t)

	// ---------------- Google OAuth
	r = call(t, "GET", "/api/v1/auth/google", "", nil)
	loc, _ := url.Parse(r.Header.Get("Location"))
	state := loc.Query().Get("state")
	var cookie *http.Cookie
	for _, c := range r.HTTP.Cookies() {
		if c.Name == "oauth_state" {
			cookie = c
		}
	}
	check(t, "/auth/google → 307 + state acak + cookie HttpOnly Secure",
		r.Status == 307 && len(state) >= 40 && cookie != nil && cookie.Value == state && cookie.HttpOnly && cookie.Secure &&
			strings.Contains(loc.Query().Get("scope"), "openid"), r.Status, loc)
	callback := func(code, st, cookieVal string) resp {
		h := []string{}
		if cookieVal != "" {
			h = append(h, "Cookie", "oauth_state="+cookieVal)
		}
		return call(t, "GET", "/api/v1/auth/google/callback?code="+code+"&state="+url.QueryEscape(st), "", nil, h...)
	}
	check(t, "callback tanpa cookie → 400", callback("good", state, "").Status == 400)
	check(t, "callback state beda → 400", callback("good", "state-lain", state).Status == 400)
	r = callback("good", state, state)
	loc, _ = url.Parse(r.Header.Get("Location"))
	frag, _ := url.ParseQuery(loc.Fragment)
	check(t, "callback sukses → FRONTEND_URL/oauth-success#access_token (bukan query)",
		r.Status == 303 && loc.Host == "shop.test" && loc.Path == "/oauth-success" && loc.RawQuery == "" &&
			frag.Get("access_token") != "" && frag.Get("refresh_token") != "", r.Status, loc)
	check(t, "user Google tersimpan dengan sub & email huruf kecil", scalar[string](t, "select provider_sub from users where email = 'budi@gmail.com'") == "g-123")
	check(t, "token dari Google bisa dipakai", call(t, "GET", "/api/v1/cart", frag.Get("access_token"), nil).Status == 200)
	callback("good", state, state)
	check(t, "login Google kedua → user yang sama", scalar[int](t, "select count(*) from users where provider_sub = 'g-123'") == 1)
	r = callback("unverified", state, state)
	loc, _ = url.Parse(r.Header.Get("Location"))
	check(t, "email Google belum terverifikasi → #error=email_unverified", strings.Contains(loc.Fragment, "error=email_unverified") &&
		scalar[int](t, "select count(*) from users where email = 'palsu@gmail.com'") == 0, loc)
	r = callback("local", state, state)
	loc, _ = url.Parse(r.Header.Get("Location"))
	check(t, "email akun password tidak diambil alih → #error=email_registered", strings.Contains(loc.Fragment, "error=email_registered"), loc)

	// ---------------- Redis mati → fail-closed
	acc, _ = login(t, "/api/v1/auth/login", "pembeli@test.local", buyerPassword)
	good := config.RedisClient
	config.RedisClient = redis.NewClient(&redis.Options{Addr: "localhost:1"})
	r = call(t, "GET", "/api/v1/cart", acc, nil)
	ready := call(t, "GET", "/readyz", "", nil)
	config.RedisClient = good
	check(t, "Redis mati → endpoint ber-auth 503", r.Status == 503, r.Status)
	check(t, "Redis mati → /readyz 503 redis unavailable", ready.Status == 503 && ready.Body["checks"].(map[string]any)["redis"] == "unavailable", ready.Status, string(ready.Raw))
}
