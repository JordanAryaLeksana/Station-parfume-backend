// Command seed menjalankan seeder di src/config/seeders (kategori, ukuran,
// wangian, harga, produk umum dari CSV di folder seed/), atau membuat akun
// owner dari env kalau pakai -admin.
//
// Koneksi dari env yang sama dengan server (DATABASE_URL, atau DB_HOST/DB_PORT/
// DB_USER/DB_PASSWORD/DB_NAME; .env dibaca kalau ada), atau flag -dsn. Database
// harus sudah dimigrasi (go run ./cmd/migrate up).
//
//	go run ./seed -dry-run        # cek dulu, semua di-rollback
//	go run ./seed                 # simpan
//	go run ./seed -admin          # HANYA buat akun owner dari ADMIN_NAME/ADMIN_EMAIL/ADMIN_PASSWORD
//
// Lihat seed/README.md untuk format CSV.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/migrations"
	"backend/src/config"
	"backend/src/config/seeders"
)

func main() {
	if _, err := config.LoadDotEnv(".env"); err != nil { // khusus dev
		log.Fatal(err)
	}
	dir := flag.String("dir", "seed", "folder berisi categories.csv, sizes.csv, wangian.csv, varian_parfum.csv, produk_umum.csv")
	dsn := flag.String("dsn", "", "DSN PostgreSQL (default: dari env, sama seperti server)")
	dryRun := flag.Bool("dry-run", false, "jalankan semua lalu rollback (tidak ada yang tersimpan)")
	withAdmin := flag.Bool("admin", false, "hanya buat akun owner (staf dengan semua akses) dari env ADMIN_NAME, ADMIN_EMAIL, ADMIN_PASSWORD (katalog dilewati)")
	flag.Parse()
	if err := checkArgs(flag.Args()); err != nil {
		log.Fatal(err)
	}

	if *dsn == "" {
		v, err := config.DatabaseDSN()
		if err != nil {
			log.Fatal(err)
		}
		*dsn = v
	}
	db, err := config.OpenDatabase(*dsn)
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = migrations.CheckUpToDate(ctx, sqlDB)
	cancel()
	if err != nil && !errors.Is(err, migrations.ErrDBNewer) {
		log.Fatalf("%v — jalankan dulu: go run ./cmd/migrate up", err)
	}

	// -admin: hanya buat admin, CSV katalog tidak dibaca (jadi tetap bisa
	// dijalankan walaupun catalog.csv masih berisi TODO).
	if *withAdmin {
		if *dryRun {
			fmt.Println("dry run: admin tidak dibuat")
			return
		}
		created, err := seeders.SeedAdmin(db)
		switch {
		case err != nil:
			log.Fatalf("seed admin: %v", err)
		case created:
			fmt.Println("owner: dibuat")
		default:
			fmt.Println("owner: email sudah terdaftar, dilewati")
		}
		return
	}

	var data seeders.CatalogData
	if data.Categories, err = load(*dir, "categories.csv", seeders.LoadCategories); err != nil {
		log.Fatal(err)
	}
	if data.Sizes, err = load(*dir, "sizes.csv", seeders.LoadSizes); err != nil {
		log.Fatal(err)
	}
	if data.Scents, err = load(*dir, "wangian.csv", seeders.LoadScents); err != nil {
		log.Fatal(err)
	}
	if data.Variants, err = load(*dir, "varian_parfum.csv", seeders.LoadPerfumeVariants); err != nil {
		log.Fatal(err)
	}
	if data.General, err = load(*dir, "produk_umum.csv", seeders.LoadGeneralProducts); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("CSV OK: %d kategori, %d ukuran, %d wangian, %d pengecualian varian parfum, %d baris produk umum\n",
		len(data.Categories), len(data.Sizes), len(data.Scents), len(data.Variants), len(data.General))

	rep, err := seeders.SeedCatalog(db, data, *dryRun)
	if rep != nil {
		fmt.Printf("kategori : %v\nukuran   : %v\nproduk   : %v\nwangian  : %v\nvarian   : %v\n"+
			"harga khusus/status diterapkan: %d baris, stok berubah: %d varian, dinonaktifkan: %d\n",
			rep.Categories, rep.Sizes, rep.Products, rep.Scents, rep.Variants, rep.PricesApplied, rep.StockChanged, rep.Archived)
		if rep.Unpriced > 0 {
			fmt.Printf("PERINGATAN: %d wangian/produk belum punya tarif atau harga (tidak tampil di website).\n", rep.Unpriced)
		}
	}
	switch {
	case errors.Is(err, seeders.ErrDryRun):
		fmt.Println("DRY RUN: tidak ada yang disimpan.")
	case err != nil:
		log.Fatalf("seed gagal, semua di-rollback: %v", err)
	default:
		fmt.Println("Seed selesai.")
	}
}

// load membaca satu CSV; file yang tidak ada dilewati (dianggap kosong).
func load[T any](dir, name string, parse func(io.Reader) ([]T, error)) ([]T, error) {
	f, err := os.Open(filepath.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("lewati %s (tidak ada)\n", name)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse(f)
}

// checkArgs: seed hanya menerima flag. Kata lepas (mis. "go run ./seed admin"
// tanpa tanda minus) dulu diam-diam diabaikan lalu seed katalog berjalan.
func checkArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	hint := ""
	switch strings.TrimLeft(args[0], "-") {
	case "admin", "dry-run", "dir", "dsn":
		hint = fmt.Sprintf(" — maksud Anda -%s?", strings.TrimLeft(args[0], "-"))
	}
	return fmt.Errorf("argumen tidak dikenal: %q%s (pemakaian: go run ./seed [-dry-run] [-admin] [-dir DIR] [-dsn DSN])", args[0], hint)
}
