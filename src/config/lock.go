package config

import (
	"context"
	"errors"
)

// TryAdvisoryLock mencoba advisory lock PostgreSQL (level sesi) tanpa
// menunggu. ok=false berarti proses lain (replika lain) sedang memegangnya.
// release wajib dipanggil kalau ok=true; lock juga lepas sendiri kalau
// koneksinya putus.
func TryAdvisoryLock(ctx context.Context, key int64) (release func(), ok bool, err error) {
	if DB == nil {
		return nil, false, errors.New("database belum terhubung")
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, false, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok); err != nil || !ok {
		conn.Close()
		return nil, false, err
	}
	return func() {
		// context.Background: lock tetap dilepas walau ctx pemanggil sudah dibatalkan.
		conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
		conn.Close()
	}, true, nil
}
