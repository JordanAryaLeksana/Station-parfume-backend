# Seed katalog (schema v2)

Seeder mengisi tabel katalog dari 3 file CSV di folder ini, supaya isi website **sama persis** dengan listing di Shopee.

Kode seeder ada di [`src/config/seeders/`](../src/config/seeders/) (sama seperti seeder lama), model GORM di [`src/repository/`](../src/repository/). [`cmd/seed`](../cmd/seed/main.go) hanya titik masuk untuk menjalankannya tanpa menyalakan server.

| File | Isi | Status |
|---|---|---|
| `aromas.csv` | 50 aroma (master data) | ✅ terisi, urutan dan ejaan persis seperti daftar dari Pilot |
| `bottles.csv` | master botol | ⏳ baru header (tidak dipakai: parfum dijual tanpa variasi botol) |
| `catalog.csv` | listing + kombinasi variasi (harga, stok, SKU) | ⏳ 50 baris siap, **judul listing dan harga masih `TODO`** |

### Mengisi `catalog.csv` (parfum disatukan, tanpa botol)

Semua parfum ada di **1 listing** dengan **1 variasi "Aroma"** berisi 50 pilihan (urutan = urutan di `aromas.csv`). Tidak ada variasi botol. Yang masih harus diisi:

1. `product_name`: ganti `TODO judul listing Shopee` di semua baris dengan judul listing persis seperti di Shopee.
2. `price`: ganti `TODO` dengan harga. Kalau semua aroma harganya sama, isi angka yang sama di semua baris.
3. `stock`: sekarang `0` (tidak bisa dibeli). Isi stok asli.
4. Opsional: `sku`, `original_price` (harga coret), `weight_gram`, `shopee_item_id`/`shopee_model_id`.

Selama masih ada `TODO` di file ini, seeder **menolak berjalan**. Jadi placeholder tidak mungkin masuk ke database.

## Menjalankan

Database harus sudah memakai [`docs/schema/schema.sql`](../docs/schema/schema.sql).

```bash
export DATABASE_URL="host=localhost port=5432 user=postgres password=... dbname=station_parfume sslmode=disable"
```

Cek dulu tanpa menyimpan apa pun:

```bash
go run ./cmd/seed -dry-run
```

Simpan:

```bash
go run ./cmd/seed
```

Buat juga akun admin (password di-hash bcrypt, dilewati kalau email sudah ada):

```bash
ADMIN_NAME="Admin" ADMIN_EMAIL="admin@example.com" ADMIN_PASSWORD="minimal-8-karakter" go run ./cmd/seed -admin
```

- Semua berjalan dalam **satu transaksi**. Kalau ada satu error, tidak ada yang tersimpan.
- Aman dijalankan berulang kali. Data yang sudah ada diperbarui dan tidak diduplikasi (dicocokkan lewat slug nama, label pilihan, dan kombinasi variasi).
- Tidak ada yang dihapus. Kombinasi yang tidak ada lagi di `catalog.csv` hanya **dinonaktifkan** (`is_active = false`). Pilihan variasinya dipindah ke urutan paling belakang.
- Seeder menampilkan **PERINGATAN** kalau ada pilihan di variasi bernama "Aroma" yang tidak ada di `aromas.csv` (biasanya typo), atau ada aroma yang belum dipakai di listing mana pun.

## `aromas.csv`

| Kolom | Wajib | Isi |
|---|---|---|
| `name` | ✅ | Nama aroma. **Harus sama persis dengan label variasi di Shopee** (huruf besar/kecil boleh beda). |
| `gender` | | `men` / `women` / `unisex` (kosong = `unisex`) |
| `grade` | | `regular` / `exclusive` / `super_exclusive` (kosong = `regular`) |
| `brand` | | brand inspirasi, mis. `Christian Dior`. Otomatis dibuat di tabel `brands`. |
| `description` | | deskripsi / notes |

> Kolom `gender`, `grade`, dan `brand` sengaja dikosongkan karena belum ada datanya. Silakan isi sendiri; jangan sampai tebakan masuk ke data.

## `bottles.csv`

| Kolom | Wajib | Contoh |
|---|---|---|
| `name` | ✅ | `Spray 30ml`. Samakan dengan label variasi ukuran di Shopee supaya tertaut otomatis. |
| `bottle_type` | ✅ | `spray`, `roll_on` |
| `volume_ml` | ✅ | `30` |
| `description` | | |

## `catalog.csv`

**Satu baris = satu kombinasi variasi** (satu baris di "Ubah Massal" Seller Center). Urutan baris menentukan urutan pilihan, jadi ikuti urutan di Shopee.

| Kolom | Wajib | Isi |
|---|---|---|
| `product_name` | ✅ | judul listing Shopee (baris dengan nama sama = 1 produk) |
| `product_type` | ✅ | `perfume` / `empty_bottle` / `other` |
| `parent_sku` | | SKU Induk |
| `variation1`, `option1` | | nama Variasi 1 dan pilihannya, mis. `Aroma`, `Victoria` |
| `variation2`, `option2` | | nama Variasi 2 dan pilihannya, mis. `Ukuran`, `Spray 30ml` |
| `sku` | | SKU kombinasi (harus unik) |
| `price` | ✅ | rupiah utuh: `45000`, `45.000`, atau `Rp45.000` (desimal/koma ditolak) |
| `original_price` | | harga coret, harus ≥ `price` |
| `stock` | ✅ | |
| `weight_gram` | | berat produk untuk ongkir |
| `shopee_item_id`, `shopee_model_id` | | ID dari Seller Center |

Contoh bentuk yang dipakai sekarang (1 listing parfum, variasi Aroma saja; judul dan harga hanya contoh):

```csv
product_name,product_type,parent_sku,variation1,option1,variation2,option2,sku,price,original_price,stock,weight_gram,shopee_item_id,shopee_model_id
Parfum Refill Station,perfume,PRS,Aroma,Victoria,,,PRS-VICTORIA,45000,,10,100,,
Parfum Refill Station,perfume,PRS,Aroma,Bvlgari,,,PRS-BVLGARI,45000,,10,,,
Parfum Refill Station,perfume,PRS,Aroma,D & G,,,PRS-DG,50000,,10,,,
```

Kalau nanti dijual per ukuran ml, tambahkan variasi kedua (`variation2=Ukuran`, `option2=30ml`) di setiap baris. Botol kosong bisa jadi listing terpisah (`product_type=empty_bottle`).

Produk tanpa variasi: kosongkan semua kolom variasi, dan produk itu hanya boleh punya 1 baris.
