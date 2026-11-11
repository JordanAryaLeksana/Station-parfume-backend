# Migrasi database (goose)

Schema PostgreSQL dikelola dengan [goose](https://github.com/pressly/goose) v3.
File `*.sql` di folder ini di-embed ke binary, jadi tidak perlu install CLI goose.

| File | Isi |
|---|---|
| `00001_init.sql` | Schema awal sesuai kebutuhan sekarang: 15 tabel, FK, CHECK, partial index. Penjelasan & ERD: [`docs/schema/README.md`](../docs/schema/README.md) |

## Perintah

Koneksi dari env yang sama dengan server: `DATABASE_URL`, atau `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME` (+ `DB_SSLMODE`, `DB_TIMEZONE`). `.env` dibaca otomatis kalau ada.

```bash
go run ./cmd/migrate status      # daftar migrasi & mana yang sudah diterapkan
go run ./cmd/migrate up          # terapkan semua yang belum (tiap migrasi 1 transaksi)
go run ./cmd/migrate version     # versi database vs versi kode
go run ./cmd/migrate down -yes   # batalkan SATU migrasi terakhir (bisa menghapus data)
```

Di Docker image: `./migrate up`.

- **Server tidak menjalankan migrasi.** Saat start, server hanya mengecek versi: database yang belum dimigrasi atau ketinggalan → server menolak start dengan pesan `jalankan dulu: go run ./cmd/migrate up`. Database yang lebih baru dari kode (mis. setelah rollback deploy) hanya diberi peringatan.
- `./seed` juga menolak database yang belum dimigrasi.
- Dua proses `up` bersamaan aman: goose memakai advisory lock PostgreSQL.

## Database dev lama

Database yang dulu dibuat manual dari `docs/schema/schema.sql` (schema v3, 26 tabel) tidak cocok dengan `00001`. Buat database baru, lalu:

```bash
go run ./cmd/migrate up
go run ./seed            # kategori, ukuran, wangian & tarif, produk umum
go run ./seed -admin     # akun owner dari ADMIN_NAME / ADMIN_EMAIL / ADMIN_PASSWORD
```

## Membuat migrasi baru

1. Buat file `0000N_deskripsi_singkat.sql` (nomor berikutnya, 5 digit):
   ```sql
   -- +goose Up
   ALTER TABLE ...;

   -- +goose Down
   ALTER TABLE ...;
   ```
2. **Jangan pernah mengubah migrasi yang sudah diterapkan di database mana pun** (termasuk dev orang lain). Perbaikan = migrasi baru.
3. Perubahan destruktif (hapus/rename kolom) dipecah: *expand* (tambah kolom baru, kode menulis keduanya) → deploy → backfill → *contract* (hapus kolom lama) di migrasi berikutnya.
4. `CREATE INDEX CONCURRENTLY` tidak bisa di dalam transaksi: beri `-- +goose NO TRANSACTION` di file tersendiri.
5. Fungsi/trigger yang berisi `;` dibungkus `-- +goose StatementBegin` … `-- +goose StatementEnd`.
6. Sesuaikan struct GORM di `src/repository/` (tag hanya untuk mapping; jangan `AutoMigrate`).
