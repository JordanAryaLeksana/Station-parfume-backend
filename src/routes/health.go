package routes

import (
	"backend/src/config"
	"context"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

var shuttingDown atomic.Bool

// SetShuttingDown dipanggil main.go saat menerima SIGTERM: /readyz langsung
// menjawab 503 supaya load balancer berhenti mengirim request baru.
func SetShuttingDown() { shuttingDown.Store(true) }

type healthBody struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// healthz (liveness): proses hidup. Tidak memeriksa dependensi.
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, healthBody{Status: "ok"})
}

// readyz (readiness): siap melayani — database & Redis bisa di-ping dan server
// tidak sedang shutdown. Detail error hanya ke log.
func readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	checks := map[string]string{"database": "ok", "redis": "ok"}
	ok := true
	fail := func(name string, err error) {
		ok = false
		checks[name] = "unavailable"
		if err != nil {
			log.Printf("readyz: %s: %v", name, err)
		}
	}
	if shuttingDown.Load() {
		ok = false
		checks["server"] = "shutting_down"
	}
	if config.DB == nil {
		fail("database", nil)
	} else if sqlDB, err := config.DB.DB(); err != nil {
		fail("database", err)
	} else if err := sqlDB.PingContext(ctx); err != nil {
		fail("database", err)
	}
	if config.RedisClient == nil {
		fail("redis", nil)
	} else if err := config.RedisClient.Ping(ctx).Err(); err != nil {
		fail("redis", err)
	}
	if !ok {
		c.JSON(http.StatusServiceUnavailable, healthBody{Status: "unavailable", Checks: checks})
		return
	}
	c.JSON(http.StatusOK, healthBody{Status: "ok", Checks: checks})
}
