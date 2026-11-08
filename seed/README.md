# Seed katalog (schema v2)

Seeder mengisi tabel katalog dari 2 file CSV di folder ini, supaya isi website **sama persis** dengan toko Shopee `shopee.co.id/stationparfume`.

Kode seeder ada di [`src/config/seeders/`](../src/config/seeders/), model GORM di [`src/repository/`](../src/repository/). [`main.go`](main.go) di folder ini hanya titik masuk untuk menjalankannya tanpa menyalakan server (`go run ./seed`).

| File | Isi | Status |
|---|---|---|
| `categories.csv` | kategori toko, persis seperti di halaman toko Shopee | ✅ terisi: 7 kategori, urutan dan ID dari Shopee (dicek 2026-11-08) |
| `catalog.csv` | listing + kombinasi variasi (kategori, harga, stok, SKU) | ⏳ 50 baris siap, **judul listing dan harga masih `TODO`** |

Tidak ada lagi `aromas.csv` dan `bottles.csv`. Di Shopee parfum dijual sebagai **satu produk** (botol + isi), jadi nama parfum ditulis di judul listing atau sebagai pilihan variasi, dan botol kosong adalah listing biasa. Ke-50 aroma tetap ada sebagai pilihan variasi "Aroma" di `catalog.csv`.

### Mengisi `catalog.csv`

Sekarang isinya 1 listing dengan variasi "Aroma" berisi 50 pilihan. **Bentuk ini belum sama dengan toko Shopee.** Di Shopee, kebanyakan parfum adalah listing sendiri, misalnya "Anggelina Jolie" dengan variasi `Warna` (Campur/Murni) × `Ukuran` (15/25/35/50 ML). Supaya website identik, isi file ini dari export Seller Center (**Produk Saya → Ubah Massal → Download**), lalu salin per baris kombinasi variasi.

Selama masih ada `TODO` di file ini, seeder **menolak berjalan**. Jadi placeholder tidak mungkin masuk ke database.

## Menjalankan

Database harus sudah memakai [`docs/schema/schema.sql`](../docs/schema/schema.sql).

```bash
export DATABASE_URL="host=localhost port=5432 user=postgres password=... dbname=station_parfume sslmode=disable"
```

Cek dulu tanpa menyimpan apa pun:

```bash
go run ./seed -dry-run
```

Simpan:

```bash
go run ./seed
```

Buat akun admin. Perintah ini **hanya** membuat admin dan tidak membaca CSV katalog, jadi bisa dijalankan walaupun `catalog.csv` masih berisi TODO. Password di-hash bcrypt, dan perintah ini dilewati kalau email sudah terdaftar. Endpoint `/admins` hanya bisa dipakai admin yang sudah login, jadi admin pertama **harus** dibuat lewat perintah ini:

```bash
ADMIN_NAME="Admin" ADMIN_EMAIL="admin@example.com" ADMIN_PASSWORD="minimal-8-karakter" go run ./seed -admin
```

- Semua berjalan dalam **satu transaksi**. Kalau ada satu error, tidak ada yang tersimpan.
- Aman dijalankan berulang kali. Data yang sudah ada diperbarui dan tidak diduplikasi (kategori dicocokkan lewat `shopee_category_id` atau nama; produk lewat slug nama; pilihan lewat label; model lewat kombinasi variasi).
- Tidak ada yang dihapus. Kombinasi yang tidak ada lagi di `catalog.csv` hanya **dinonaktifkan** (`is_active = false`). Pilihan variasinya dipindah ke urutan paling belakang.
- Seeder menampilkan **PERINGATAN** untuk kategori yang belum punya produk di `catalog.csv`.

## `categories.csv`

Urutan baris = urutan tampil (sama seperti di Shopee).

| Kolom | Wajib | Isi |
|---|---|---|
| `name` | ✅ | nama kategori persis seperti di Shopee, mis. `Parfum & Wewangian` |
| `shopee_category_id` | | ID kategori toko di Shopee (dari URL `?originalCategoryId=…`), mis. `11043198` |

## `catalog.csv`

**Satu baris = satu kombinasi variasi** (satu baris di "Ubah Massal" Seller Center). Urutan baris menentukan urutan pilihan, jadi ikuti urutan di Shopee.

| Kolom | Wajib | Isi |
|---|---|---|
| `product_name` | ✅ | judul listing Shopee (baris dengan nama sama = 1 produk) |
| `category` | ✅ | nama kategori, harus ada di `categories.csv` (huruf besar/kecil boleh beda) |
| `parent_sku` | | SKU Induk |
| `variation1`, `option1` | | nama Variasi 1 dan pilihannya, mis. `Warna`, `Murni` |
| `variation2`, `option2` | | nama Variasi 2 dan pilihannya, mis. `Ukuran`, `25 ML` |
| `sku` | | SKU kombinasi (harus unik) |
| `price` | ✅ | rupiah utuh: `45000`, `45.000`, atau `Rp45.000` (desimal/koma ditolak) |
| `original_price` | | harga coret, harus ≥ `price` |
| `stock` | ✅ | stok kombinasi ini (sama seperti di Shopee) |
| `weight_gram` | | berat produk untuk ongkir |
| `shopee_item_id`, `shopee_model_id` | | ID dari Seller Center |

Contoh dengan bentuk listing yang ada di toko (harga dan stok hanya contoh):

```csv
product_name,category,parent_sku,variation1,option1,variation2,option2,sku,price,original_price,stock,weight_gram,shopee_item_id,shopee_model_id
MASTER Inspired parfume by MAHERZEIN,Parfum & Wewangian,,Warna,Campur,Ukuran,15 ML,MZ-C-15,155000,,5,,7413921292,
MASTER Inspired parfume by MAHERZEIN,Parfum & Wewangian,,Warna,Murni,Ukuran,15 ML,MZ-M-15,185000,,0,,7413921292,
Botol spray,Kecantikan Lainnya,,,,,,BTL-SPRAY,5000,,40,30,,
```

Produk tanpa variasi (seperti "Botol spray"): kosongkan semua kolom variasi, dan produk itu hanya boleh punya 1 baris. Kategori `Kecantikan Lainnya` di contoh hanya ilustrasi, jadi cek kategori asli listing itu di Shopee.
