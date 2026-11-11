package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	write := func(content string) string {
		p := filepath.Join(dir, ".env")
		os.WriteFile(p, []byte(content), 0o600)
		return p
	}
	if found, err := LoadDotEnv(filepath.Join(dir, "tidak-ada")); found || err != nil {
		t.Fatalf("file tidak ada: %v %v", found, err)
	}
	t.Setenv("DOTENV_TEST_A", "")
	os.Unsetenv("DOTENV_TEST_A")
	p := write("# komentar\n\nDOTENV_TEST_A=satu\nexport DOTENV_TEST_B=\"dua\"\nDOTENV_TEST_C=\"baris1\nbaris2\"\n")
	t.Cleanup(func() { os.Unsetenv("DOTENV_TEST_A"); os.Unsetenv("DOTENV_TEST_B"); os.Unsetenv("DOTENV_TEST_C") })
	if found, err := LoadDotEnv(p); !found || err != nil || os.Getenv("DOTENV_TEST_A") != "satu" || os.Getenv("DOTENV_TEST_B") != "dua" {
		t.Fatalf("file valid: %v %v a=%q b=%q", found, err, os.Getenv("DOTENV_TEST_A"), os.Getenv("DOTENV_TEST_B"))
	}
	p = write("DB_HOST=localhost\n# Redis\n Server\nDB_PASSWORD=RAHASIA-JANGAN-BOCOR\n")
	_, err := LoadDotEnv(p)
	if err == nil || !strings.Contains(err.Error(), "baris 3") {
		t.Fatalf("baris tanpa '=' harus error dengan nomor baris: %v", err)
	}
	if strings.Contains(err.Error(), "Server") || strings.Contains(err.Error(), "RAHASIA") {
		t.Fatalf("pesan error tidak boleh memuat isi file: %v", err)
	}
}
