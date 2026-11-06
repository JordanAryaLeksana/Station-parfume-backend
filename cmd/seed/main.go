// Command seed menjalankan seeder di src/config/seeders (katalog dari CSV
// di folder seed/, dan admin dari env kalau pakai -admin).
//
//	DATABASE_URL="host=localhost user=postgres password=... dbname=station_parfume sslmode=disable" \
//	  go run ./cmd/seed -dry-run      # cek dulu, semua di-rollback
//	go run ./cmd/seed                 # simpan
//	go run ./cmd/seed -admin          # + buat admin dari ADMIN_NAME/ADMIN_EMAIL/ADMIN_PASSWORD
//
// Lihat seed/README.md untuk format CSV.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"backend/src/config/seeders"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dir := flag.String("dir", "seed", "folder berisi aromas.csv, bottles.csv, catalog.csv")
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "DSN PostgreSQL (default: env DATABASE_URL)")
	dryRun := flag.Bool("dry-run", false, "jalankan semua lalu rollback (tidak ada yang tersimpan)")
	withAdmin := flag.Bool("admin", false, "buat akun admin dari env ADMIN_NAME, ADMIN_EMAIL, ADMIN_PASSWORD")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("DATABASE_URL kosong: set env DATABASE_URL atau pakai -dsn")
	}

	var data seeders.CatalogData
	var err error
	if data.Aromas, err = load(*dir, "aromas.csv", seeders.LoadAromas); err != nil {
		log.Fatal(err)
	}
	if data.Bottles, err = load(*dir, "bottles.csv", seeders.LoadBottles); err != nil {
		log.Fatal(err)
	}
	if data.Catalog, err = load(*dir, "catalog.csv", seeders.LoadCatalog); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("CSV OK: %d aroma, %d botol, %d model\n", len(data.Aromas), len(data.Bottles), len(data.Catalog))

	gormLog := logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true, // "belum ada" adalah alur normal upsert
	})
	db, err := gorm.Open(postgres.Open(*dsn), &gorm.Config{Logger: gormLog})
	if err != nil {
		log.Fatalf("koneksi DB: %v", err)
	}

	if *withAdmin {
		if *dryRun {
			fmt.Println("lewati admin (dry run)")
		} else if created, err := seeders.SeedAdmin(db); err != nil {
			log.Fatalf("seed admin: %v", err)
		} else if created {
			fmt.Println("admin: dibuat")
		} else {
			fmt.Println("admin: sudah ada, dilewati")
		}
	}

	rep, err := seeders.SeedCatalog(db, data, *dryRun)
	if rep != nil {
		fmt.Printf("brand    : %v\naroma    : %v\nbotol    : %v\nproduk   : %v\nvariasi  : %v\npilihan  : %v\nmodel/SKU: %v, %d dinonaktifkan\n",
			rep.Brands, rep.Aromas, rep.Bottles, rep.Products, rep.Variations, rep.Options, rep.Models, rep.ModelsDeactivated)
		for _, w := range rep.Warnings {
			fmt.Println("PERINGATAN:", w)
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
