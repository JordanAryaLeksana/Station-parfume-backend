// Package ratelimit: batas jumlah request per IP untuk endpoint sensitif
// (login, register, refresh) memakai counter Redis.
package ratelimit

import (
	"backend/src/config"
	httperror "backend/src/middlewares/Error"
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Limit membatasi max request per window per IP (c.ClientIP(), yang hanya
// membaca X-Forwarded-For dari proxy di TRUSTED_PROXIES). Lewat batas → 429
// + Retry-After. Kalau Redis error, request DILOLOSKAN (rate limit tidak boleh
// mematikan login), dan error dicatat di log.
func Limit(prefix string, max int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rl:%s:%s", prefix, c.ClientIP())
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		pipe := config.RedisClient.TxPipeline()
		incr := pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, window) // TTL hanya dipasang saat key baru dibuat
		ttl := pipe.TTL(ctx, key)
		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("rate limit %s: redis error, request diloloskan: %v", prefix, err)
			c.Next()
			return
		}
		if incr.Val() > int64(max) {
			retry := int(ttl.Val().Seconds())
			if retry < 1 {
				retry = 1
			}
			c.Header("Retry-After", strconv.Itoa(retry))
			httperror.TooManyRequestsError(c, "Terlalu banyak percobaan. Coba lagi nanti.")
			return
		}
		c.Next()
	}
}
