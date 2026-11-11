package config

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // zona waktu (DB_TIMEZONE) tetap ada di image tanpa tzdata

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

var sslModes = []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

// DatabaseDSN menyusun DSN PostgreSQL dari env. Kalau DATABASE_URL diisi, itu
// yang dipakai apa adanya. Kalau tidak, DSN dirakit dari:
//
//	DB_HOST, DB_USER, DB_NAME  (wajib)
//	DB_PASSWORD                (opsional, kosong = tanpa password)
//	DB_PORT                    (default 5432)
//	DB_SSLMODE                 (default prefer; production: require / verify-full)
//	DB_TIMEZONE                (default Asia/Jakarta)
//
// Nilai di-escape lewat net/url, jadi password boleh berisi karakter apa pun.
// Pesan error tidak pernah memuat password.
func DatabaseDSN() (string, error) {
	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		return v, nil
	}
	host := strings.TrimSpace(os.Getenv("DB_HOST"))
	user := strings.TrimSpace(os.Getenv("DB_USER"))
	name := strings.TrimSpace(os.Getenv("DB_NAME"))
	var missing []string
	for k, v := range map[string]string{"DB_HOST": host, "DB_USER": user, "DB_NAME": name} {
		if v == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return "", fmt.Errorf("database belum dikonfigurasi: isi DATABASE_URL, atau %s", strings.Join(missing, ", "))
	}
	port := envOr("DB_PORT", "5432")
	if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
		return "", errors.New("DB_PORT harus angka 1-65535")
	}
	sslmode := envOr("DB_SSLMODE", "prefer")
	if !slices.Contains(sslModes, sslmode) {
		return "", fmt.Errorf("DB_SSLMODE tidak dikenal (pilih: %s)", strings.Join(sslModes, ", "))
	}
	u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(host, port), Path: "/" + name}
	if pass := os.Getenv("DB_PASSWORD"); pass != "" {
		u.User = url.UserPassword(user, pass)
	} else {
		u.User = url.User(user)
	}
	tz := envOr("DB_TIMEZONE", "Asia/Jakarta")
	if !tzRe.MatchString(tz) {
		return "", errors.New("DB_TIMEZONE harus nama zona IANA, mis. Asia/Jakarta atau UTC")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return "", fmt.Errorf("DB_TIMEZONE tidak dikenal: %q", tz)
	}
	// TimeZone sengaja TIDAK di-escape: driver GORM postgres membaca nilainya
	// mentah dari DSN ("Asia%2FJakarta" akan ditolak). Aman karena sudah
	// dibatasi tzRe; "/" boleh tampil apa adanya di query URL.
	u.RawQuery = "sslmode=" + sslmode + "&TimeZone=" + tz
	return u.String(), nil
}

var tzRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-]*(/[A-Za-z0-9_+\-]+)*$`)

// OpenDatabase membuka koneksi + pool. Schema TIDAK dibuat/diubah di sini:
// migrasi dijalankan terpisah (go run ./cmd/migrate up, lihat migrations/).
//
// Logger GORM level Warn (DB_LOG_LEVEL=silent|error|warn|info) dan selalu
// tanpa nilai parameter, supaya data pengguna tidak masuk log.
func OpenDatabase(dsn string) (*gorm.DB, error) {
	level, err := gormLogLevel(os.Getenv("DB_LOG_LEVEL"))
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  level,
			IgnoreRecordNotFoundError: true, // "tidak ada" adalah alur normal, bukan error
			ParameterizedQueries:      true, // jangan cetak nilai parameter (email, alamat, dll.)
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("koneksi database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}

// ConnectDatabase membaca DSN dari env, membuka koneksi, dan mengisi config.DB.
func ConnectDatabase() error {
	dsn, err := DatabaseDSN()
	if err != nil {
		return err
	}
	db, err := OpenDatabase(dsn)
	if err != nil {
		return err
	}
	DB = db
	log.Println("Database connection established successfully")
	return nil
}

func gormLogLevel(s string) (logger.LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "warn":
		return logger.Warn, nil
	case "silent":
		return logger.Silent, nil
	case "error":
		return logger.Error, nil
	case "info":
		return logger.Info, nil
	}
	return 0, errors.New("DB_LOG_LEVEL tidak dikenal (pilih: silent, error, warn, info)")
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
