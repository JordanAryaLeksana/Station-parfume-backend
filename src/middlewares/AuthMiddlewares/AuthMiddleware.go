package authmiddlewares

import (
	"backend/src/config"
	httperror "backend/src/middlewares/Error"
	"backend/src/repository"
	"backend/src/utils/jwt"
	utils "backend/src/utils/redisUtils"
	"errors"
	"log"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Context keys yang diisi middleware.
const (
	CtxUserID        = "userID"
	CtxEmail         = "email"
	CtxRole          = "role"
	CtxJTI           = "jti"           // jti access token (untuk logout)
	CtxExpiresAt     = "tokenExpires"  // time.Time exp access token
	CtxRefreshClaims = "refreshClaims" // *jwt.JwtCustomClaims (endpoint /auth/refresh)
)

// bearerToken mengambil token dari header "Authorization: Bearer <token>".
func bearerToken(c *gin.Context) (string, bool) {
	h := c.GetHeader("Authorization")
	scheme, tok, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(tok) == "" {
		return "", false
	}
	return strings.TrimSpace(tok), true
}

// checkNotRevoked: fail-closed — kalau Redis error, request ditolak 503.
func checkNotRevoked(c *gin.Context, jti string) bool {
	revoked, err := utils.IsJtiBlacklisted(jti)
	if err != nil {
		log.Printf("auth: cek blacklist gagal: %v", err)
		httperror.ServiceUnavailableError(c, "Layanan autentikasi sedang bermasalah, coba lagi")
		return false
	}
	if revoked {
		httperror.UnauthorizedError(c, "Token sudah tidak berlaku")
		return false
	}
	return true
}

// AuthMiddleware memverifikasi access token (HS256, issuer, tipe "access", jti,
// exp) dan memastikan token belum dicabut (logout).
func AuthMiddleware(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, ok := bearerToken(c)
		if !ok {
			httperror.UnauthorizedError(c, "Authorization header tidak ada atau formatnya salah")
			return
		}
		claims, err := jwt.ParseAccessToken(tok, secretKey)
		if err != nil {
			httperror.UnauthorizedError(c, "Invalid token")
			return
		}
		if !checkNotRevoked(c, claims.ID) {
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxEmail, claims.Email)
		c.Set(CtxRole, claims.Role)
		c.Set(CtxJTI, claims.ID)
		c.Set(CtxExpiresAt, claims.ExpiresAt.Time)
		c.Next()
	}
}

// RefreshTokenMiddleware memverifikasi refresh token (tipe "refresh") dan
// menaruh claims-nya di context untuk handler /auth/refresh.
func RefreshTokenMiddleware(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, ok := bearerToken(c)
		if !ok {
			httperror.UnauthorizedError(c, "Authorization header tidak ada atau formatnya salah")
			return
		}
		claims, err := jwt.ParseRefreshToken(tok, secretKey)
		if err != nil {
			httperror.UnauthorizedError(c, "Invalid token")
			return
		}
		if !checkNotRevoked(c, claims.ID) {
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRefreshClaims, claims)
		c.Next()
	}
}

// RoleBasedAccess harus dipasang SETELAH AuthMiddleware dan SEBELUM handler.
// Peran dibaca ULANG dari database (bukan dari token), jadi staf yang dihapus
// atau diturunkan perannya langsung kehilangan akses tanpa menunggu token
// kedaluwarsa. Context "role" diperbarui ke peran terbaru.
func RoleBasedAccess(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint(CtxUserID)
		if userID == 0 {
			httperror.UnauthorizedError(c, "Unauthorized")
			return
		}
		var u repository.User
		err := config.DB.Select("id", "role").First(&u, userID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httperror.UnauthorizedError(c, "Akun tidak aktif")
			return
		}
		if err != nil {
			log.Printf("auth: baca peran user %d gagal: %v", userID, err)
			httperror.InternalServerError(c, "Internal server error")
			return
		}
		if !slices.Contains(allowedRoles, u.Role) {
			httperror.ForbiddenError(c, "You don't have permission to access this resource")
			return
		}
		c.Set(CtxRole, u.Role)
		c.Next()
	}
}
