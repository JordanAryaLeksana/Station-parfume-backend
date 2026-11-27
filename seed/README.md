# Seed katalog (schema v3)

Seeder mengisi katalog dari file CSV di folder ini. Semua file bisa dibuka dan diisi lewat **Excel / Google Sheets** (simpan sebagai CSV). Kode seeder ada di [`src/config/seeders/`](../src/config/seeders/); [`main.go`](main.go) di folder ini adalah titik masuknya.

## Rumus harga parfum

Harga parfum **tidak diketik per varian**, tapi dihitung:

```
harga = tarif per ml wangian × ukuran ml + harga botol ukuran

Bibit  = Murni   → tarif_murni
Parfum = Campur  → tarif_campur

Victoria 30 ml Murni  = 3.000 × 30 + 5.000 = Rp95.000
Victoria 30 ml Campur = 1.500 × 30 + 5.000 = Rp50.000
Moonlight (Exclusive) 50 ml Murni = 4.000 × 50 + 5.000 = Rp205.000
```

Harga khusus per varian (promo, ukuran spesial) opsional di `varian_parfum.csv` → mengalahkan rumus.

**Stok parfum tidak dihitung** (diracik sendiri, selalu tersedia): `products.track_stock = false`. Kalau bibit suatu wangian habis, admin menonaktifkan wangian atau variannya. Produk umum (barang jadi) tetap memakai stok; pengaturan ini bisa diubah per produk dari admin.

| File | Isi | Status |
|---|---|---|
| `categories.csv` | kategori toko | ✅ 7 kategori |
| `sizes.csv` | ukuran botol + harga botol + default | ✅ 20 / 30 / 50 ml, botol Rp5.000 (berat kirim belum diisi) |
| `wangian.csv` | produk parfum (1 baris = 1 parfum = 1 produk) + tarif per ml | ✅ 92 parfum: 81 Regular (73 aktif) + 11 Exclusive, tarif dari daftar harga 2026-11-10 |
| `varian_parfum.csv` | pengecualian per varian parfum: harga khusus, harga coret, nonaktif | ✅ kosong (tidak wajib diisi) |
| `produk_umum.csv` | produk non-parfum (botol spray, souvenir, parfum mobil, Al Rehab, …) | ✅ 16 produk, harga dari Shopee; ⚠️ 3 souvenir belum ada harga, **stok masih kosong** (produk umum memakai stok) |

Dunhil (Desire Blue/London/Red), Greentea, Guci Flora, dan Hugo Boss belum ada di daftar harga → disimpan sebagai **draft** (`aktif = tidak`), tidak tampil di website.

## Menjalankan

Database harus sudah dimigrasi ([`migrations/`](../migrations/README.md)): `go run ./cmd/migrate up`. Seeder menolak database yang belum dimigrasi.

Koneksi dari env yang sama dengan server (`.env` dibaca otomatis): `DATABASE_URL`, atau `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`. Bisa juga `-dsn "postgres://..."`.

Cek dulu tanpa menyimpan apa pun:

```bash
go run ./seed -dry-run
```

Simpan:

```bash
go run ./seed
```

Buat akun **owner** (staf dengan semua akses). Perintah ini hanya membuat owner dan tidak membaca CSV. Login staf lewat `POST /api/v1/auth/login/admin`.

```bash
ADMIN_NAME="Owner" ADMIN_EMAIL="owner@example.com" ADMIN_PASSWORD="minimal-8-karakter" go run ./seed -admin
```

Aturan seeder:

- Semua dalam **satu transaksi**: ada 1 error → tidak ada yang tersimpan.
- Aman dijalankan berulang. Data dicocokkan lewat kode (`group_code`, `code`, `product_code`), ukuran lewat `volume_ml`.
- **Kolom kosong = tidak diubah.** Tarif, harga khusus, dan stok yang sudah diisi admin lewat website tidak terhapus kalau seeder dijalankan lagi dengan kolom kosong.
- Setiap wangian otomatis dibuatkan semua kombinasi ukuran (default atau khusus produk) × Murni/Campur, langsung berharga sesuai rumus.
- Tidak ada yang dihapus. Wangian/produk parfum yang tidak ada lagi di `wangian.csv` dinonaktifkan.
- Wangian **tidak bisa dipindah** ke produk lain: `code` yang sudah tersimpan terikat ke produknya. Seeder menolak dengan pesan "sudah milik produk lain" dan tidak menyimpan apa pun.
- Stok produk umum yang diisi tercatat di riwayat stok (`stock_movements`).

### Perubahan 2026-11-20: 1 parfum = 1 produk (reset database sekali)

Sebelumnya 1 produk = 1 grup foto (mis. "Kasturi" berisi Kasturi Kijang & Kasturi Putih). Sekarang setiap parfum adalah produk sendiri ("Kasturi Kijang", "Kasturi Putih"). Kode wangian (`code`) tidak berubah, tetapi produknya berubah, jadi database yang pernah di-seed dengan format lama harus **di-reset sekali**. Langkah ini **menghapus semua data, termasuk akun**, jadi hanya untuk database yang belum berisi pesanan asli:

```bash
go run ./cmd/migrate status       # catat jumlah migrasi yang terpasang
go run ./cmd/migrate down -yes    # ulangi sampai semua migrasi pending (1x = 1 migrasi)
go run ./cmd/migrate up
go run ./seed
ADMIN_NAME="Owner" ADMIN_EMAIL="owner@example.com" ADMIN_PASSWORD="minimal-8-karakter" go run ./seed -admin
```

Database baru (belum pernah di-seed) tidak perlu reset.

## `wangian.csv`

1 baris = 1 parfum = 1 produk (1 halaman). Urutan baris = urutan produk di website (sekarang A–Z menurut nama).

Seeder sebenarnya masih bisa menggabungkan beberapa baris dengan `group_code` yang sama menjadi 1 produk berisi beberapa wangian. Katalog tidak memakai cara itu lagi, dan test `TestSeedFilesAreConsistent` menjaga agar setiap `group_code` unik.

| Kolom | Isi |
|---|---|
| `grade` | `regular` / `exclusive`. Nama sama di grade berbeda = **2 parfum berbeda** (mis. Bacarat Regular 4.000/ml vs Bacarat Exclusive 5.000/ml), dengan alamat berbeda: `bacarat` dan `bacarat-exclusive` |
| `group_code` | kode produk, sekaligus alamat halaman, unik: huruf kecil, angka, dan `-` (mis. `kasturi-kijang` → `/produk/kasturi-kijang`). **Jangan diubah** setelah dipakai |
| `group`, `category` | nama produk (mis. "Kasturi Kijang"), kategori |
| `note1`, `note2` | 2 label aroma: amber, aromatic, balsamic, citrus, creamy, floral, fresh, fruity, musk, oud, powdery, sweet, woody |
| `tarif_murni` | tarif **bibit** Rp/ml. Kosong = tidak diubah (wangian baru tanpa tarif tidak dijual) |
| `tarif_campur` | tarif **parfum** Rp/ml |
| `ukuran` | kosong = ukuran default (20/30/50). Isi mis. `10;15;30` kalau produk ini hanya dijual di ukuran itu (harus ada di `sizes.csv`) |
| `aktif` | kosong = aktif; `tidak` = produk draft (tidak tampil) |
| `code` | kode wangian, unik. **Jangan diubah** setelah dipakai (juga kunci `varian_parfum.csv`) |
| `varian`, `image` | nama wangian persis seperti di foto (mis. "Blue" untuk Jaguar Blue), nama file foto (sementara masih poster grup lama sampai foto per parfum datang) |

## `internal/inspired_by.csv` — khusus admin, tidak di-commit

Repo ini publik, jadi pasangan produk dengan merek parfum aslinya tidak boleh ada di file yang di-commit. Kolom `inspired_by` tidak lagi ada di `wangian.csv`; seeder menolak `wangian.csv` yang masih memuatnya.

| Kolom | Isi |
|---|---|
| `code` | `group_code` produk di `wangian.csv` |
| `inspired_by` | teks "Inspired by …", maksimal 255 karakter. Hanya terlihat di endpoint admin; tidak dikirim ke website dan tidak ikut pencarian publik |

- Folder `seed/internal/` ada di `.gitignore`. Simpan salinannya di luar repo.
- File tidak ada: `inspired_by` di database tidak diubah (isian admin aman).
- `code` yang tidak ada di `wangian.csv` membuat seed gagal, jadi setelah ganti nama `code` di file ini harus ikut diganti.

## `sizes.csv`

| Kolom | Isi |
|---|---|
| `label`, `volume_ml` | "30 ml", 30 (unik) |
| `bottle_price` | harga botol, ditambahkan ke harga Murni & Campur |
| `weight_gram` | berat kirim untuk ongkir |
| `default` | `ya` = dijual untuk semua parfum. Ukuran tanpa `ya` (mis. 10 ml) hanya dipakai produk yang mencantumkannya di kolom `ukuran` |

## `varian_parfum.csv` — pengecualian (opsional)

Biasanya kosong. Isi 1 baris hanya untuk varian yang berbeda dari aturan umum. Kunci: `scent_code` + `ml` + `tipe`. Kolom `product` dan `varian` hanya supaya mudah dibaca.

| Kolom | Isi |
|---|---|
| `scent_code` | kode wangian dari `wangian.csv` |
| `ml` | ukuran yang dijual untuk produk itu |
| `tipe` | `murni`/`bibit` atau `campur`/`parfum` |
| `harga_khusus` | mengalahkan rumus. Kosong = tidak diubah |
| `harga_coret` | opsional |
| `aktif` | `tidak` = sembunyikan kombinasi ini (mis. bibit habis). Kosong = tidak diubah |
| `stok` | opsional; hanya dipakai kalau produknya diatur `track_stock` |

```csv
product,varian,scent_code,ml,tipe,harga_khusus,harga_coret,aktif
Annasui Dolly Girl,Dolly Girl,annasui-dolly-girl,30,murni,85000,95000,
Annasui Sweet Dream,Sweet Dream,annasui-sweet-dream,50,campur,,,tidak
```

## `produk_umum.csv`

1 baris = 1 pilihan. Baris dengan `product_code` sama = 1 produk. Produk tanpa pilihan = 1 baris dengan `label` kosong. Harga produk umum diisi langsung (`harga`).

| Kolom | Isi |
|---|---|
| `product_code` | kode produk, huruf kecil + `-` |
| `product_name`, `category` | nama & kategori |
| `label` | nama pilihan ("Pakai foto", "Lemon"); kosong kalau tidak ada pilihan |
| `harga`, `stok`, `aktif` | kosong = tidak diubah |
| `weight_gram` | berat kirim |

Catatan data dari Shopee (2026-11-08): harga 3 souvenir per pilihan belum diketahui (kosong), dan listing "PARFUM MOBIL" punya 15 pilihan di Shopee tapi baru 12 yang tercatat.
