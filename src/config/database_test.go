package config

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Sama persis dengan timeZoneMatcher di gorm.io/driver/postgres: driver itu
// mengambil TimeZone MENTAH dari DSN lalu time.LoadLocation saat konek, jadi
// nilai yang di-URL-escape ("Asia%2FJakarta") baru gagal ketika konek ke DB.
var gormTimeZoneMatcher = regexp.MustCompile("(time_zone|TimeZone|timezone)=(.*?)($|&| )")

func assertGormTimeZone(t *testing.T, dsn, want string) {
	t.Helper()
	m := gormTimeZoneMatcher.FindStringSubmatch(dsn)
	if len(m) < 3 || m[2] != want {
		t.Fatalf("driver GORM akan membaca TimeZone %q, harus %q", m, want)
	}
	if _, err := time.LoadLocation(m[2]); err != nil {
		t.Fatalf("driver GORM akan gagal konek: %v", err)
	}
}

func clearDBEnv(t *testing.T) {
	for _, k := range []string{"DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "DB_TIMEZONE"} {
		t.Setenv(k, "")
	}
}

func TestDatabaseDSN_FromParts(t *testing.T) {
	clearDBEnv(t)
	pass := `p@ss:w/rd ?#&='"\%`
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "station parfume")
	t.Setenv("DB_PASSWORD", pass)
	t.Setenv("DB_NAME", "station-parfume-db")
	dsn, err := DatabaseDSN()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("DSN tidak bisa di-parse pgx: %v", err)
	}
	if cfg.Host != "db.internal" || cfg.Port != 5433 || cfg.User != "station parfume" ||
		cfg.Password != pass || cfg.Database != "station-parfume-db" {
		t.Fatalf("hasil parse salah: host=%q port=%d user=%q db=%q passOK=%v",
			cfg.Host, cfg.Port, cfg.User, cfg.Database, cfg.Password == pass)
	}
	if cfg.RuntimeParams["TimeZone"] != "Asia/Jakarta" {
		t.Fatalf("TimeZone default = %q", cfg.RuntimeParams["TimeZone"])
	}
	if cfg.TLSConfig == nil || len(cfg.Fallbacks) == 0 {
		t.Fatal("sslmode default harus prefer (coba TLS, boleh jatuh ke tanpa TLS)")
	}
	assertGormTimeZone(t, dsn, "Asia/Jakarta")
}

func TestDatabaseDSN_TimeZone(t *testing.T) {
	clearDBEnv(t)
	t.Setenv("DB_HOST", "h")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_NAME", "d")
	for _, tz := range []string{"UTC", "Asia/Makassar", "America/Argentina/Buenos_Aires", "Etc/GMT+7"} {
		t.Setenv("DB_TIMEZONE", tz)
		dsn, err := DatabaseDSN()
		if err != nil {
			t.Fatalf("%s: %v", tz, err)
		}
		assertGormTimeZone(t, dsn, tz)
	}
	for _, tz := range []string{"Asia/Nowhere", "UTC&sslmode=disable", "../etc/passwd", "Asia Jakarta"} {
		t.Setenv("DB_TIMEZONE", tz)
		if _, err := DatabaseDSN(); err == nil {
			t.Fatalf("%q harus ditolak", tz)
		}
	}
}

func TestDatabaseDSN_URLWins(t *testing.T) {
	clearDBEnv(t)
	t.Setenv("DATABASE_URL", "postgres://a@b/c")
	t.Setenv("DB_HOST", "lain")
	if dsn, err := DatabaseDSN(); err != nil || dsn != "postgres://a@b/c" {
		t.Fatalf("DATABASE_URL harus dipakai apa adanya: %q %v", dsn, err)
	}
}

func TestDatabaseDSN_Errors(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"kosong", nil, "DB_HOST, DB_NAME, DB_USER"},
		{"tanpa nama db", map[string]string{"DB_HOST": "h", "DB_USER": "u"}, "DB_NAME"},
		{"port bukan angka", map[string]string{"DB_HOST": "h", "DB_USER": "u", "DB_NAME": "d", "DB_PORT": "abc"}, "DB_PORT"},
		{"port di luar batas", map[string]string{"DB_HOST": "h", "DB_USER": "u", "DB_NAME": "d", "DB_PORT": "70000"}, "DB_PORT"},
		{"sslmode salah", map[string]string{"DB_HOST": "h", "DB_USER": "u", "DB_NAME": "d", "DB_SSLMODE": "yes"}, "DB_SSLMODE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearDBEnv(t)
			t.Setenv("DB_PASSWORD", "RAHASIA-JANGAN-BOCOR")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := DatabaseDSN()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, harus menyebut %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "RAHASIA") {
				t.Fatal("pesan error memuat password")
			}
		})
	}
}

func TestGormLogLevel(t *testing.T) {
	for _, s := range []string{"", "warn", "WARN", "silent", "error", "info"} {
		if _, err := gormLogLevel(s); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
	}
	if _, err := gormLogLevel("debug"); err == nil {
		t.Fatal("level tidak dikenal harus error")
	}
}
