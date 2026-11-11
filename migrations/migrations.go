// Package migrations berisi migrasi SQL (goose) yang di-embed ke binary, plus
// helper untuk menjalankan dan mengecek versinya. Perintahnya ada di
// cmd/migrate; lihat migrations/README.md.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
	"github.com/pressly/goose/v3/lock"
)

//go:embed *.sql
var FS embed.FS

var (
	// ErrNotMigrated: tabel versi goose belum ada (database belum pernah dimigrasi).
	ErrNotMigrated = errors.New("database belum dimigrasi")
	// ErrPending: ada migrasi yang belum diterapkan.
	ErrPending = errors.New("ada migrasi yang belum diterapkan")
	// ErrDBNewer: database sudah di versi yang lebih baru dari kode ini
	// (mis. setelah rollback deploy). Bukan fatal, tapi perlu dicek.
	ErrDBNewer = errors.New("versi database lebih baru dari kode")
)

func store() (database.StoreExtender, error) {
	s, err := database.NewStore(database.DialectPostgres, goose.DefaultTablename)
	if err != nil {
		return nil, err
	}
	ext, ok := s.(database.StoreExtender)
	if !ok {
		return nil, errors.New("store goose postgres tidak mendukung TableExists")
	}
	return ext, nil
}

// NewProvider membuat provider goose untuk PostgreSQL. Dengan session lock
// (advisory lock), dua proses migrate yang jalan bersamaan tidak saling tabrak.
func NewProvider(db *sql.DB, opts ...goose.ProviderOption) (*goose.Provider, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, err
	}
	opts = append([]goose.ProviderOption{goose.WithSessionLocker(locker)}, opts...)
	return goose.NewProvider(goose.DialectPostgres, db, FS, opts...)
}

// CheckUpToDate dipanggil saat server start: hanya membaca, tidak pernah
// membuat tabel atau menjalankan migrasi.
func CheckUpToDate(ctx context.Context, db *sql.DB) error {
	st, err := store()
	if err != nil {
		return err
	}
	exists, err := st.TableExists(ctx, db)
	if err != nil {
		return fmt.Errorf("cek tabel versi migrasi: %w", err)
	}
	if !exists {
		return ErrNotMigrated
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, FS)
	if err != nil {
		return err
	}
	current, target, err := p.GetVersions(ctx)
	if err != nil {
		return err
	}
	pending, err := p.HasPending(ctx)
	if err != nil {
		return err
	}
	switch {
	case pending:
		return fmt.Errorf("%w: database di versi %d, kode butuh %d", ErrPending, current, target)
	case current > target:
		return fmt.Errorf("%w: database di versi %d, kode hanya kenal sampai %d", ErrDBNewer, current, target)
	}
	return nil
}
