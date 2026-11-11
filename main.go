package main

import (
	"backend/migrations"
	"backend/src/config"
	paymentservices "backend/src/modules/payment/services"
	"backend/src/routes"
	authhandler "backend/src/utils/AuthHandler"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// .env (khusus dev) dimuat PALING AWAL, sebelum config apa pun membaca env.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found (memakai environment variable sistem)")
	}
	// GIN_MODE dari .env baru berlaku setelah .env dimuat.
	if err := config.ApplyGinMode(); err != nil {
		log.Fatal(err)
	}
	// Fail-fast: tanpa secret JWT yang kuat, siapa pun bisa memalsukan token.
	if err := config.LoadAuthConfig(); err != nil {
		log.Fatalf("config auth tidak valid: %v", err)
	}
	// Redis wajib: blacklist token (logout) dan rate limit bergantung padanya.
	if err := config.InitRedis(); err != nil {
		log.Fatalf("redis: %v", err)
	}
	// Database dari env (DATABASE_URL atau DB_*), lalu pastikan schema sudah di
	// versi migrasi terbaru. Migrasi sendiri dijalankan terpisah:
	// go run ./cmd/migrate up (lihat migrations/README.md).
	if err := config.ConnectDatabase(); err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := checkSchema(); err != nil {
		log.Fatalf("database: %v", err)
	}
	// Pembayaran: Xendit Payment Sessions. Kalau env belum lengkap, server tetap
	// jalan tapi endpoint bayar & webhook menjawab 503.
	if err := paymentservices.ConfigureFromEnv(); err != nil {
		log.Printf("PERINGATAN pembayaran: %v", err)
	}
	origins, err := config.CORSOrigins()
	if err != nil {
		log.Fatal(err)
	}
	if len(origins) == 0 {
		log.Println("PERINGATAN CORS: CORS_ALLOWED_ORIGINS/FRONTEND_URL kosong — frontend di domain lain akan ditolak")
	}
	docs, err := config.DocsEnabled()
	if err != nil {
		log.Fatal(err)
	}
	authhandler.InitGoogleOauthConfig()
	// Hanya proxy di TRUSTED_PROXIES yang boleh menentukan IP klien
	// (X-Forwarded-For); default tidak ada → IP dari koneksi langsung.
	engine, err := routes.NewEngine(routes.Options{CORSOrigins: origins, TrustedProxies: config.TrustedProxies(), Docs: docs})
	if err != nil {
		log.Fatalf("router: %v", err)
	}

	// SIGINT/SIGTERM → graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Seeder (src/config/seeders) dijalankan terpisah: go run ./seed — lihat seed/README.md.
	jobsDone := make(chan struct{})
	go func() {
		defer close(jobsDone)
		runJobs(ctx, 5*time.Minute)
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           routes.Handler(engine),
		ReadHeaderTimeout: 5 * time.Second,  // klien lambat (slowloris) tidak bisa menahan koneksi
		ReadTimeout:       15 * time.Second, // body maks 1 MiB
		WriteTimeout:      30 * time.Second, // panggilan Xendit dibatasi 10 dtk
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("server mendengarkan di :%s (docs: %v)", port, docs)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	case <-ctx.Done():
	}

	log.Println("shutdown: berhenti menerima request baru, menunggu request yang berjalan…")
	routes.SetShuttingDown()
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	select {
	case <-jobsDone:
	case <-time.After(5 * time.Second):
		log.Println("shutdown: job latar belakang belum selesai, ditinggal")
	}
	if err := config.RedisClient.Close(); err != nil {
		log.Printf("shutdown redis: %v", err)
	}
	if sqlDB, err := config.DB.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Printf("shutdown database: %v", err)
		}
	}
	log.Println("shutdown selesai")
}

// checkSchema: server tidak jalan di atas schema yang belum dimigrasi (query
// akan gagal di tengah jalan). Database yang lebih baru dari kode (mis. setelah
// rollback deploy) hanya diberi peringatan, karena migrasi dibuat kompatibel ke
// belakang (expand → contract).
func checkSchema() error {
	sqlDB, err := config.DB.DB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = migrations.CheckUpToDate(ctx, sqlDB)
	switch {
	case errors.Is(err, migrations.ErrDBNewer):
		log.Printf("PERINGATAN schema: %v", err)
		return nil
	case errors.Is(err, migrations.ErrNotMigrated), errors.Is(err, migrations.ErrPending):
		return fmt.Errorf("%w — jalankan dulu: go run ./cmd/migrate up", err)
	}
	return err
}

// jobsLockKey: advisory lock PostgreSQL untuk job latar belakang, supaya
// kalau server dijalankan lebih dari 1 replika hanya satu yang menjalankannya.
const jobsLockKey int64 = 0x53504a4f4253 // "SPJOBS"

// runJobs: langsung jalan sekali, lalu setiap `every` sampai ctx dibatalkan (shutdown).
func runJobs(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		runJobsOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// runJobsOnce: cek pembayaran yang webhook-nya terlewat, lalu order yang lewat
// batas bayar dibatalkan sesi Xendit-nya dan ditandai EXPIRED (reservasi stok
// dilepas). Panic ditangkap supaya tidak mematikan server.
func runJobsOnce(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("job latar belakang panic: %v\n%s", r, debug.Stack())
		}
	}()
	release, ok, err := config.TryAdvisoryLock(ctx, jobsLockKey)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("job latar belakang: lock: %v", err)
		}
		return
	}
	if !ok {
		return // replika lain sedang menjalankan
	}
	defer release()
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if n, err := paymentservices.ReconcileAndExpire(runCtx, time.Now()); err != nil {
		log.Printf("rekonsiliasi pembayaran: %v", err)
	} else if n > 0 {
		log.Printf("rekonsiliasi pembayaran: %d order kedaluwarsa", n)
	}
}
