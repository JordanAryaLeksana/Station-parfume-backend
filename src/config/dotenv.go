package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

var dotenvLine = regexp.MustCompile(`^(export\s+)?[A-Za-z_][A-Za-z0-9_.]*\s*=`)

// LoadDotEnv memuat file .env (khusus development) kalau ada.
//
//   - Tidak ada file → found=false, err=nil (pakai environment variable sistem).
//   - File ada tapi ada baris yang bukan KEY=VALUE / komentar (#) → error dengan
//     NOMOR BARIS saja; isi baris tidak dicetak supaya secret tidak bocor ke log.
//
// Variabel yang sudah ada di environment tidak ditimpa.
func LoadDotEnv(path string) (found bool, err error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	inQuote := false // nilai multi-baris dalam kutip ganda
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if inQuote {
			if strings.HasSuffix(line, `"`) {
				inQuote = false
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !dotenvLine.MatchString(line) {
			return true, fmt.Errorf("%s baris %d tidak valid: harus KEY=VALUE, komentar diawali #", path, n)
		}
		if v := strings.TrimSpace(line[strings.Index(line, "=")+1:]); strings.HasPrefix(v, `"`) && (len(v) == 1 || !strings.HasSuffix(v, `"`)) {
			inQuote = true
		}
	}
	if err := sc.Err(); err != nil {
		return true, err
	}
	if err := godotenv.Load(path); err != nil {
		// Pesan godotenv bisa memuat potongan isi file (secret) — jangan diteruskan.
		return true, fmt.Errorf("%s tidak bisa dibaca: periksa tanda kutip dan format KEY=VALUE", path)
	}
	return true, nil
}
