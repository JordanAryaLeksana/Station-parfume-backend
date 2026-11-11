package services

import (
	"backend/src/config"
	"backend/src/modules/auth/models"
	"backend/src/repository"
	authhandler "backend/src/utils/AuthHandler"
	"backend/src/utils/jwt"
	utils "backend/src/utils/redisUtils"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var validate = validator.New()

var (
	// ErrInvalidCredentials: SATU pesan untuk semua kegagalan login (email tidak
	// terdaftar, password salah, akun Google, bukan staf) supaya tidak bisa
	// dipakai menebak email/akun staf.
	ErrInvalidCredentials = errors.New("Email atau password salah")
	ErrValidation         = errors.New("validation error")
	ErrConflict           = errors.New("Email sudah terdaftar")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrUnavailable        = errors.New("layanan autentikasi sedang bermasalah")
)

func Register(input *models.RegisterRequest) (*models.RegisterResponse, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	email := repository.NormalizeEmail(input.Email)
	var n int64
	if err := config.DB.Model(&repository.User{}).Where("lower(email) = ?", email).Count(&n).Error; err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrConflict
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	hash := string(hashed)
	u := repository.User{Name: strings.TrimSpace(input.Name), Email: email, PasswordHash: &hash,
		Role: repository.RoleUser, Provider: repository.ProviderLocal}
	if err := config.DB.Create(&u).Error; err != nil {
		if isUniqueViolation(err) { // daftar bersamaan dengan email yang sama
			return nil, ErrConflict
		}
		return nil, err
	}
	return &models.RegisterResponse{Name: u.Name, Email: u.Email, Role: u.Role, Sub: u.ProviderSub, Provider: u.Provider}, nil
}

func isUniqueViolation(err error) bool {
	s := err.Error()
	return strings.Contains(s, "23505") || strings.Contains(s, "duplicate key")
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// burnPasswordCheck menjalankan bcrypt pada hash palsu supaya waktu respons
// "email tidak terdaftar" sama dengan "password salah" (anti enumerasi).
func burnPasswordCheck(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-untuk-timing"), bcrypt.DefaultCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

// LoginManual: login pembeli (dan staf) dengan email + password.
func LoginManual(input *models.LoginRequest) (*models.LoginResponse, error) {
	return login(input, false)
}

// LoginAdmin: login staf (owner, admin, gudang, CS). Pembeli ditolak dengan
// pesan yang SAMA seperti password salah.
func LoginAdmin(input *models.LoginRequest) (*models.LoginResponse, error) {
	return login(input, true)
}

func login(input *models.LoginRequest, staffOnly bool) (*models.LoginResponse, error) {
	var u repository.User
	err := config.DB.Where("lower(email) = ?", repository.NormalizeEmail(input.Email)).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		burnPasswordCheck(input.Password)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if u.Provider != repository.ProviderLocal || u.PasswordHash == nil {
		burnPasswordCheck(input.Password)
		return nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(input.Password)) != nil {
		return nil, ErrInvalidCredentials
	}
	if staffOnly && !slices.Contains(repository.StaffRoles, u.Role) {
		return nil, ErrInvalidCredentials
	}
	return issueTokens(u)
}

func issueTokens(u repository.User) (*models.LoginResponse, error) {
	access, err := jwt.GenerateAccessToken(u)
	if err != nil {
		return nil, err
	}
	refresh, err := jwt.GenerateRefreshToken(u)
	if err != nil {
		return nil, err
	}
	picture := ""
	if u.PictureURL != nil {
		picture = *u.PictureURL
	}
	return &models.LoginResponse{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, Picture: picture,
		Provider: u.Provider, Sub: u.ProviderSub, AccessToken: access, RefreshToken: refresh}, nil
}

// Logout mencabut access token yang sedang dipakai (jti dari middleware) dan,
// kalau dikirim, refresh token milik user yang sama. Refresh token yang tidak
// valid/milik orang lain diabaikan (logout access token tetap berhasil).
func Logout(userID uint, accessJTI string, accessExp time.Time, refreshToken string) error {
	if err := utils.BlacklistJti(accessJTI, time.Until(accessExp)); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if refreshToken == "" {
		return nil
	}
	claims, err := jwt.ParseRefreshToken(refreshToken, os.Getenv("JWT_REFRESH_SECRET"))
	if err != nil || claims.UserID != userID {
		return nil
	}
	if err := utils.BlacklistJti(claims.ID, time.Until(claims.ExpiresAt.Time)); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

// RefreshToken menukar refresh token dengan pasangan token baru. Refresh token
// lama "dikonsumsi" secara atomik (SET NX) sebelum token baru dibuat, jadi
// tidak bisa dipakai dua kali — termasuk oleh dua request bersamaan.
func RefreshToken(claims *jwt.JwtCustomClaims) (*models.PairToken, error) {
	fresh, err := utils.ConsumeJti(claims.ID, time.Until(claims.ExpiresAt.Time))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if !fresh {
		return nil, fmt.Errorf("%w: refresh token sudah dipakai", ErrUnauthorized)
	}
	var u repository.User // peran terbaru dari DB; user terhapus → ditolak
	if err := config.DB.First(&u, claims.UserID).Error; err != nil {
		return nil, fmt.Errorf("%w: akun tidak aktif", ErrUnauthorized)
	}
	access, err := jwt.GenerateAccessToken(u)
	if err != nil {
		return nil, err
	}
	refresh, err := jwt.GenerateRefreshToken(u)
	if err != nil {
		return nil, err
	}
	return &models.PairToken{AccessToken: access, RefreshToken: refresh}, nil
}

// ---------------------------------------------------------------- Google OAuth

const stateCookie = "oauth_state"

func cookieSecure() bool {
	return strings.HasPrefix(authhandler.GoogleOauthConfig.RedirectURL, "https://")
}

// HandleGoogleLogin: buat state acak, simpan di cookie HttpOnly (10 menit),
// lalu redirect ke halaman persetujuan Google.
func HandleGoogleLogin(c *gin.Context) {
	if !authhandler.GoogleConfigured() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Login Google belum dikonfigurasi"})
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	state := base64.RawURLEncoding.EncodeToString(b)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, state, 600, "/", "", cookieSecure(), true)
	c.Redirect(http.StatusTemporaryRedirect, authhandler.GoogleOauthConfig.AuthCodeURL(state))
}

// HandleGoogleCallback: cocokkan state (anti CSRF), tukar code, ambil userinfo
// OIDC (wajib email_verified), lalu redirect ke FRONTEND_URL/oauth-success dengan
// token di FRAGMENT (#...), bukan query — fragment tidak terkirim ke server,
// tidak masuk log, dan tidak ikut di header Referer.
func HandleGoogleCallback(c *gin.Context) {
	if !authhandler.GoogleConfigured() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Login Google belum dikonfigurasi"})
		return
	}
	cookie, _ := c.Cookie(stateCookie)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, "", -1, "/", "", cookieSecure(), true) // sekali pakai
	state := c.Query("state")
	if cookie == "" || state == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(state)) != 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "State OAuth tidak valid, ulangi login"})
		return
	}
	if c.Query("error") != "" || c.Query("code") == "" {
		redirectOAuth(c, url.Values{"error": {"access_denied"}})
		return
	}
	info, err := fetchGoogleUser(c.Request.Context(), c.Query("code"))
	if err != nil {
		log.Printf("google oauth: %v", err)
		redirectOAuth(c, url.Values{"error": {"google_failed"}})
		return
	}
	if !info.EmailVerified || info.Sub == "" || info.Email == "" {
		redirectOAuth(c, url.Values{"error": {"email_unverified"}})
		return
	}
	u, code, err := upsertGoogleUser(info)
	if err != nil {
		log.Printf("google oauth: simpan user: %v", err)
		redirectOAuth(c, url.Values{"error": {"server_error"}})
		return
	}
	if code != "" {
		redirectOAuth(c, url.Values{"error": {code}})
		return
	}
	tokens, err := issueTokens(*u)
	if err != nil {
		redirectOAuth(c, url.Values{"error": {"server_error"}})
		return
	}
	redirectOAuth(c, url.Values{"access_token": {tokens.AccessToken}, "refresh_token": {tokens.RefreshToken}})
}

func fetchGoogleUser(ctx context.Context, code string) (*models.GoogleUserInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tok, err := authhandler.GoogleOauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	resp, err := authhandler.GoogleOauthConfig.Client(ctx, tok).Get(authhandler.UserInfoURL)
	if err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo: status %d", resp.StatusCode)
	}
	var info models.GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	return &info, nil
}

// upsertGoogleUser mencari user lewat sub Google, lalu lewat email. Email yang
// sudah terdaftar sebagai akun password tidak digabung otomatis (code
// "email_registered"). Kode != "" = ditolak dengan alasan itu.
func upsertGoogleUser(info *models.GoogleUserInfo) (*repository.User, string, error) {
	var u repository.User
	err := config.DB.Where("provider = ? AND provider_sub = ?", repository.ProviderGoogle, info.Sub).First(&u).Error
	if err == nil {
		return &u, "", nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}
	email := repository.NormalizeEmail(info.Email)
	err = config.DB.Where("lower(email) = ?", email).First(&u).Error
	switch {
	case err == nil && u.Provider != repository.ProviderGoogle:
		return nil, "email_registered", nil
	case err == nil: // akun Google lama yang belum menyimpan sub
		if u.ProviderSub != nil && *u.ProviderSub != info.Sub {
			return nil, "email_registered", nil
		}
		if err := config.DB.Model(&u).Update("provider_sub", info.Sub).Error; err != nil {
			return nil, "", err
		}
		return &u, "", nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, "", err
	}
	sub := info.Sub
	var picture *string
	if info.Picture != "" {
		picture = &info.Picture
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = email
	}
	u = repository.User{Name: name, Email: email, PictureURL: picture, ProviderSub: &sub,
		Provider: repository.ProviderGoogle, Role: repository.RoleUser}
	if err := config.DB.Create(&u).Error; err != nil {
		return nil, "", err
	}
	return &u, "", nil
}

func redirectOAuth(c *gin.Context, fragment url.Values) {
	c.Redirect(http.StatusSeeOther, authhandler.FrontendURL()+"/oauth-success#"+fragment.Encode())
}
