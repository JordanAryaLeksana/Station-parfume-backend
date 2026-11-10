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
| `wangian.csv` | produk parfum (1 grup foto) + wangian + tarif per ml | ✅ 53 grup Regular (49 aktif) + 4 grup Exclusive (foto baru), 92 wangian, tarif dari daftar harga 2026-11-10 |
| `varian_parfum.csv` | pengecualian per varian parfum: harga khusus, harga coret, nonaktif | ✅ kosong (tidak wajib diisi) |
| `produk_umum.csv` | produk non-parfum (botol spray, souvenir, parfum mobil, Al Rehab, …) | ✅ 16 produk, harga dari Shopee; ⚠️ 3 souvenir belum ada harga, **stok masih kosong** (produk umum memakai stok) |

Dunhil (Desire Blue/London/Red), Greentea, Guci Flora, dan Hugo Boss belum ada di daftar harga → disimpan sebagai **draft** (`aktif = tidak`), tidak tampil di website.

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
- Stok produk umum yang diisi tercatat di riwayat stok (`stock_movements`).

## `wangian.csv`

1 baris = 1 wangian (bullet di foto). Baris dengan `group_code` sama = 1 produk (1 halaman). Grup tanpa varian (mis. "1000 Bunga") = 1 baris dengan `varian` = nama grup.

| Kolom | Isi |
|---|---|
| `grade` | `regular` / `exclusive`. Nama sama di grade berbeda = **2 parfum berbeda** (mis. Bacarat Regular 4.000/ml vs Bacarat Exclusive 5.000/ml) |
| `group_code` | kode produk, mis. `annasui`, `exclusive-3`. Juga jadi alamat halaman (`/products/annasui`) |
| `group`, `category`, `inspired_by` | nama produk di foto, kategori, teks "Inspired by …" |
| `note1`, `note2` | 2 label aroma: amber, aromatic, balsamic, citrus, creamy, floral, fresh, fruity, musk, oud, powdery, sweet, woody |
| `tarif_murni` | tarif **bibit** Rp/ml. Kosong = tidak diubah (wangian baru tanpa tarif tidak dijual) |
| `tarif_campur` | tarif **parfum** Rp/ml |
| `ukuran` | kosong = ukuran default (20/30/50). Isi mis. `10;15;30` kalau produk ini hanya dijual di ukuran itu (harus ada di `sizes.csv`). Sama untuk semua baris 1 grup |
| `aktif` | kosong = aktif; `tidak` = produk draft (tidak tampil). Sama untuk semua baris 1 grup |
| `code` | kode wangian, unik. **Jangan diubah** setelah dipakai |
| `varian`, `image` | nama wangian persis seperti di foto, nama file foto |

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
Annasui,Dolly Girl,annasui-dolly-girl,30,murni,85000,95000,
Annasui,Sweet Dream,annasui-sweet-dream,50,campur,,,tidak
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
