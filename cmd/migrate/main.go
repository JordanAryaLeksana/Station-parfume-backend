// Command migrate menjalankan migrasi database (goose, file di migrations/).
// Migrasi adalah langkah deploy terpisah: server TIDAK menjalankannya sendiri,
// hanya menolak start kalau database belum di versi terbaru.
//
//	go run ./cmd/migrate status          # daftar migrasi & yang sudah diterapkan
//	go run ./cmd/migrate up              # terapkan semua migrasi yang belum
//	go run ./cmd/migrate version         # versi database sekarang vs versi kode
//	go run ./cmd/migrate down -yes       # batalkan SATU migrasi terakhir
//
// Koneksi dari env yang sama dengan server (DATABASE_URL, atau DB_HOST/DB_PORT/
// DB_USER/DB_PASSWORD/DB_NAME; .env dibaca kalau ada), atau flag -dsn.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"backend/migrations"
	"backend/src/config"

	"github.com/pressly/goose/v3"
)

func usage() {
	fmt.Fprintln(os.Stderr, `pemakaian: go run ./cmd/migrate [-dsn DSN] <perintah>

perintah:
  status     daftar migrasi dan statusnya
  version    versi database sekarang dan versi terbaru di kode
  up         terapkan semua migrasi yang belum
  down -yes  batalkan satu migrasi terakhir (bisa menghapus data!)`)
	os.Exit(2)
}

func main() {
	if _, err := config.LoadDotEnv(".env"); err != nil { // khusus dev
		log.Fatal(err)
	}
	dsnFlag := flag.String("dsn", "", "DSN PostgreSQL (default: dari env, sama seperti server)")
	yes := flag.Bool("yes", false, "konfirmasi untuk perintah down")
	flag.Usage = usage
	flag.Parse()
	// izinkan flag setelah perintah: "down -yes"
	args := flag.Args()
	if len(args) == 0 {
		usage()
	}
	cmd := args[0]
	if err := flag.CommandLine.Parse(args[1:]); err != nil || flag.NArg() > 0 {
		usage()
	}

	dsn := *dsnFlag
	if dsn == "" {
		var err error
		if dsn, err = config.DatabaseDSN(); err != nil {
			log.Fatal(err)
		}
	}
	gdb, err := config.OpenDatabase(dsn)
	if err != nil {
		log.Fatal(err)
	}
	db, err := gdb.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	switch cmd {
	case "status", "version", "up", "down":
	default:
		usage()
	}

	p, err := migrations.NewProvider(db)
	if err != nil {
		log.Fatal(err)
	}
	switch cmd {
	case "status":
		list, err := p.Status(ctx)
		if err != nil {
			log.Fatal(err)
		}
		for _, s := range list {
			when := "belum diterapkan"
			if s.State == goose.StateApplied {
				when = "diterapkan " + s.AppliedAt.Local().Format("2006-01-02 15:04")
			}
			fmt.Printf("%05d  %-40s %s\n", s.Source.Version, path.Base(s.Source.Path), when)
		}
	case "version":
		current, target, err := p.GetVersions(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("database: %d, kode: %d\n", current, target)
	case "up":
		res, err := p.Up(ctx)
		printResults(res)
		if err != nil {
			log.Fatalf("up gagal (migrasi yang gagal di-rollback): %v", err)
		}
		if len(res) == 0 {
			fmt.Println("tidak ada migrasi baru; database sudah terbaru")
		}
	case "down":
		if !*yes {
			log.Fatal("down membatalkan migrasi terakhir dan bisa menghapus data. Ulangi dengan: down -yes")
		}
		r, err := p.Down(ctx)
		if r != nil {
			printResults([]*goose.MigrationResult{r})
		}
		if err != nil {
			log.Fatalf("down gagal: %v", err)
		}
	}
}

func printResults(res []*goose.MigrationResult) {
	for _, r := range res {
		status := "OK"
		if r.Error != nil {
			status = "GAGAL: " + r.Error.Error()
		}
		fmt.Printf("%-4s %05d  %-40s %v  %s\n", r.Direction, r.Source.Version, path.Base(r.Source.Path), r.Duration.Round(time.Millisecond), status)
	}
}
