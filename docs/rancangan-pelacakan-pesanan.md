# Rancangan pelacakan & monitoring pesanan — DITUNDA

> **Status (2026-11-11):** ditunda oleh Pilot untuk didiskusikan dengan owner. Belum ada kode di `master`.
> Kode setengah jadi (migrasi `00002_order_tracking.sql`, aturan SLA, daftar kurir) disimpan di `git stash`
> dengan pesan *"WIP tugas 2: pelacakan pesanan (ditunda — diskusi dengan owner)"*. Migrasi 00002 belum pernah
> dijalankan di database mana pun.
>
> Isi: (1) riset pengiriman toko parfum + aturan kurir untuk parfum beralkohol, (2) rancangan sistem pelacakan
> ala Shopee, (3) rincian implementasinya. Bahan diskusi ada di **"Pertanyaan untuk owner"** (bagian 1).
>
> Catatan dari riset: daftar kurir di bagian 2.4 masih memuat J&T, ID Express, dan Ninja Xpress — ketiganya
> menurut syarat tertulisnya melarang parfum beralkohol, jadi harus disesuaikan sebelum diimplementasikan.

## 1. Riset pengiriman toko parfum

Diminta Pilot: riset sistem pengiriman toko parfum online terkenal, memakai REA (Chrome headless, profil sementara, hanya halaman publik). Ringkasan ini ikut masuk ke dokumen Langkah 0.

**Temuan paling penting:** parfum beralkohol termasuk **barang berbahaya kelas 3** (cairan mudah terbakar, UN1266). Pengiriman lewat udara diatur PM 32/2022. Setiap kurir punya aturan sendiri:

| Kurir | Parfum beralkohol? | Catatan |
|---|---|---|
| **Lion Parcel** | ✅ Ya, bersyarat (tertulis) | REGPACK/JAGOPACK (udara) atau BIGPACK (darat); lampirkan MSDS; email detail barang sebelum kirim |
| **JNE** | ⚠️ Bersyarat, halaman resmi saling bertentangan | Barang berbahaya via REG/OKE: biaya +200%, minimal 10 kg; JTR lewat darat; asuransi **tidak menanggung kaca & barang mudah terbakar**. Minta konfirmasi tertulis cabang |
| **J&T Express** | ❌ Pada praktiknya dilarang | "Parfume" masuk kelas 3; cairan hanya lewat darat, tidak antarpulau; klaim rusak maks 3 hari + video unboxing |
| **ID Express** | ❌ Dilarang | "parfum/cologne" disebut cairan mudah terbakar |
| **Ninja Xpress** | ❌ Dilarang | "produk lain yang mengandung alkohol" |
| **SiCepat, AnterAja, Pos** | ❓ Belum jelas | Tanyakan tertulis sebelum dipakai |
| **GoSend / GrabExpress / Lalamove** | ❌ / ❓ | Grab & Lalamove melarang barang mudah terbakar → **ambil di toko** sebagai opsi lokal yang aman |

**Asuransi kurir hampir tidak membantu.** Kaca dan barang mudah terbakar dikecualikan, dan ganti rugi tanpa asuransi maksimal ±10× ongkir / Rp1 juta. Toko perlu dana cadangan sendiri untuk botol pecah/hilang.

**Pola toko parfum internasional** (FragranceNet, FragranceX, Sephora, Notino, Le Labo, Byredo):
- Parfum dikirim **darat saja** (hazmat).
- Ada wilayah yang tidak dilayani.
- Gratis ongkir di atas ambang tertentu ($49–59).
- Waktu proses 1–2 hari kerja diumumkan.
- Klaim rusak/bocor wajib **video/foto** dan dilaporkan dalam 5–7 hari; kemasan disimpan.
- Email pelacakan dikirim saat barang berangkat.

**Menyambungkan pengiriman ke website (nanti, kalau dibutuhkan):**
- **Biteship:** gratis sampai 100 kiriman/bulan, cek ongkir Rp5, lacak Rp10, ada webhook status. Pilihan lain: KiriminAja (bayar per kiriman), RajaOngkir/Komerce, Shipper.
- **Aturan barang berbahaya kurir tetap berlaku lewat agregator.**
- **Notifikasi:** sekarang cukup email + link `wa.me` berisi resi. Nanti bisa WhatsApp Business API resmi (±USD 0,025/pesan). Hindari gateway WA tidak resmi.

**Rekomendasi untuk dibahas dengan owner:**
1. Pilih kurir berdasarkan aturan tertulis: Lion Parcel utama, JNE hanya dengan konfirmasi cabang. Minta **MSDS** ke pemasok alkohol/bibit.
2. Standar packing: tutup disegel, plastik per botol, bubble wrap ≥3 lapis, kardus pas, label MUDAH PECAH, video saat packing.
3. Kebijakan klaim pembeli: video unboxing tanpa potong, lapor ≤2×24 jam, kemasan disimpan 5 hari. Toko mengganti dulu, lalu klaim ke kurir.
4. Umumkan waktu proses, jam cut-off, libur Lebaran, estimasi per wilayah, dan wilayah yang tidak dilayani.
5. **Ambil di toko** sebagai opsi gratis yang aman.
6. Untuk rancangan pelacakan (tugas 2): daftar kurir disesuaikan (J&T/ID Express/Ninja dikeluarkan untuk parfum), dan setiap kurir/layanan diberi atribut seperti "boleh parfum", "wajib MSDS", "darat saja", "batas klaim".

**Situs toko parfum Indonesia (dibaca dengan REA, profil sementara, halaman publik saja):**
- **Yang punya web store sendiri hanya Saff & Co., Carl & Claire, dan Alchemist** (semuanya Shopify).
- HMNS (domain tidak aktif), Mykonos, Onix, dan Evangeline hanya berjualan lewat marketplace/retailer.
- Halaman FAQ/pengiriman Sephora ID tidak bisa dibuka REA, jadi datanya hanya dari cuplikan pencarian.

| Toko | Gratis ongkir | Proses & cut-off | Pelacakan | Klaim rusak |
|---|---|---|---|---|
| **Saff & Co.** | Tergantung berat/jarak; minimal 159K (belum pasti) | Kemas 1–2 hari kerja; pesanan sebelum **14.00** dikirim hari itu | Halaman "Pesanan" di akun: status "Diproses" → "Dikirim" + resi, lalu cek di situs kurir. Status "Pending" dijelaskan (alamat salah, promo besar) | **≤14 hari**, wajib **video unboxing paket tersegel + label + packing list**. Rusak ringan: voucher Rp25 rb atau ½ harga. Rusak berat: ganti barang, ongkir ditanggung toko |
| **Carl & Claire** | Gratis di atas **Rp450.000** | Info pengiriman dalam 1 hari kerja; Tokopedia: same-day sebelum 14.00, tidak kirim Minggu/libur | Info pengiriman dikirim (saluran tidak disebut) | Garansi uang kembali 30 hari; ongkir ditanggung pembeli; maks 3 item |
| **Alchemist** | Subsidi maks **Rp20.000** untuk belanja ≥ Rp449.000 | Belum bayar batal otomatis: 24 jam (transfer) / 30 menit (e-wallet, kartu) | Ada FAQ "How can I track my order?" | Hanya tukar barang rusak/salah kirim, **≤7 hari kerja**. Ada **biaya DG handling** (barang berbahaya) yang dibayar pembeli |
| **Sephora ID** | ≥ Rp500.000 (angka di situs tidak konsisten) | Tidak kirim Sabtu/Minggu/libur; estimasi tiba tampil di halaman produk | — | — |
| **HMNS** (Tokopedia) | — | Lunas sebelum **12.00** dikirim hari itu; tidak kirim Sabtu/Minggu/libur | — | — |

**Pola umum toko parfum lokal:**
1. Ada jam **cut-off** (12.00–14.00) dan hari tanpa pengiriman.
2. Gratis ongkir memakai **ambang belanja atau subsidi maksimal**, bukan gratis tanpa syarat. Station Parfume sekarang menanggung semua ongkir; ini yang perlu dibahas dengan owner.
3. Kurir domestik jarang disebut di halaman kebijakan.
4. **Pelacakan cukup sederhana:** status + resi di halaman pesanan, lalu pembeli mengecek di situs kurir. Ini sama dengan fase A rancangan tugas 2.
5. Klaim rusak wajib bukti (video unboxing) dengan batas waktu pendek, dan sudah jelas siapa yang menanggung ongkir retur.
6. Dukungan lewat WhatsApp/email.
7. Tidak ada yang menjelaskan packing atau asuransi. Hanya Alchemist yang menyinggung biaya barang berbahaya.

**Pertanyaan untuk owner** (masuk dokumen Langkah 0):
- Tetap gratis ongkir semua, atau pakai ambang/subsidi?
- Kurir mana yang dipakai: Lion Parcel / JNE darat?
- Apakah ada MSDS dari pemasok?
- Jam cut-off dan hari libur kirim.
- Opsi ambil di toko.
- Kebijakan klaim (batas hari, bukti, voucher/ganti).
- Cukup "status + resi + link kurir" dulu, atau langsung pelacakan otomatis?

## 2. Rancangan pelacakan & monitoring pesanan (ala Shopee)

### 2.1 Cara kerja Shopee & Tokopedia (yang kita tiru)
**Shopee.** Sumber:
- Help Centre (help.shopee.co.id).
- Seller Education Hub (seller.shopee.co.id/edu).
- Salinan dokumentasi Open Platform (`get_order_detail`, `get_tracking_info`, `cancel_order`, `set_note`, `get_late_orders`).

**Status API:**
- UNPAID → READY_TO_SHIP → PROCESSED → SHIPPED → TO_CONFIRM_RECEIVE → COMPLETED.
- Lainnya: RETRY_SHIP (pickup gagal), IN_CANCEL (pembeli mengajukan batal), CANCELLED, TO_RETURN.
- `cancel_by` = buyer / seller / system / Ops.

**Tab penjual "Pesanan Saya":**
- Belum Bayar
- Perlu Dikirim, dengan sub-tab Perlu Diproses / Telah Diproses
- Dikirim
- Selesai
- Halaman terpisah "Pengembalian/Pembatalan" (Pembatalan, Pengembalian, Pengiriman Gagal).

**Cara penjual memantau:**
- Cari berdasarkan No. Pesanan, Nama Pembeli, Produk, No. Resi.
- Urutkan berdasarkan Tanggal Dibuat atau **Batas Pengiriman**.
- Kolom **"Hitungan Mundur"** ke batas kirim.
- "Periksa Rincian" berisi info pembeli, pembayaran, **riwayat pesanan**, dan riwayat transaksi.
- Ekspor Excel.
- Catatan privat penjual (`set_note`).
- Daftar pesanan terlambat (`get_late_orders`: batas kirim, terlambat berapa hari).

**Tab pembeli:**
- Belum Bayar
- Dikemas
- Dikirim
- Selesai
- Dibatalkan
- Pengembalian

**"Jasa Kirim Toko"** (penjual mengirim sendiri, kurir tidak terintegrasi) adalah model yang **paling cocok untuk Station Parfume**:
- Penjual menyerahkan paket ke kurir lalu mengisi resi manual.
- Status pindah ke "Dikirim" walaupun tanpa resi.
- Tidak ada pelacakan di aplikasi.
- Batas kirim = **hari kerja berikutnya** jam 23.59.

**Aturan waktu lain:**
- **Reguler:** pesanan sebelum 12.00 WIB dikirim hari itu; setelahnya, hari kerja berikutnya.
- **Hari kerja:** Minggu dan libur nasional tidak dihitung.
- **Terlambat:** dikirim di hari ke-3 dihitung terlambat; tidak dikirim di hari ke-4 dibatalkan otomatis.
- **Batas bayar:** VA 1×24 jam; e-wallet 1 jam.

**Pembatalan oleh pembeli:**
- Sebelum penjual mengatur pengiriman: "Pembatalan Instan".
- Sesudahnya: pengajuan, dan penjual punya 1×24 jam untuk merespons.
- Alasan pembatalan oleh penjual: stok habis, permintaan pembeli, alamat tidak terjangkau.

**Penyelesaian:**
- Dana dilepas 2 hari setelah paket tiba.
- **Sejak 17 Sep 2026 semua pesanan diselesaikan otomatis oleh sistem**, dan tombol "Pesanan Selesai" tidak lagi ditampilkan ke pembeli.

**Metrik "Kesehatan Toko":**
| Metrik | Definisi | Target |
|---|---|---|
| Tingkat Pesanan Tidak Terselesaikan | Batal oleh penjual + batal otomatis karena telat + retur kesalahan penjual, per 7 hari | ≤10% |
| Tingkat Keterlambatan Pengiriman | Diserahkan ke kurir setelah batas, per 7 hari | ideal <2%; poin penalti ≥10% |
| Masa Pengemasan | Rata-rata waktu kemas, per 30 hari, tanpa Minggu/libur | — |
| Tingkat pembatalan | — | <5% (contoh API) |
| Tingkat retur | — | <5% (contoh API) |

**Event pelacakan** (`tracking_info[]`): `update_time`, `description`, `logistics_status`. Ini bentuk yang kita tiru di `order_events`.

**Tokopedia / TikTok Shop** (Seller Center resmi):
- **Tab penjual:**
  - Semua pesanan
  - Belum dibayar
  - Menunggu pengiriman
  - Menunggu pengambilan
  - Sedang transit
  - Sudah diantar
  - Ditangguhkan
  - Selesai
  - Dibatalkan
- **Status API:** `UNPAID → AWAITING_SHIPMENT → AWAITING_COLLECTION → IN_TRANSIT → DELIVERED → COMPLETED` (+ `CANCELLED`, `ON_HOLD`).
- **Label untuk pembeli:**
  - Menunggu Konfirmasi
  - Pesanan Diproses
  - Pesanan Dikirim
  - Pesanan Tiba
  - Pesanan Selesai
  - Pesanan Dikomplain
  - Pesanan Dibatalkan
- **Aturan waktu:**
  - Pesanan sebelum jam 12:00 wajib dikirim hari itu juga; setelah jam 12:00, besoknya (hari kerja).
  - Pesanan otomatis selesai **6 hari** setelah "Tiba di tujuan", atau saat pembeli menekan "Selesaikan Pesanan".
  - Pengajuan batal dari pembeli yang tidak direspons 48 jam → batal.

**Pelacakan kurir:**
- Kurir besar (JNE, J&T, SiCepat, AnterAja, Ninja, Pos, ID Express, Lion) **tidak punya link lacak resmi** yang bisa diisi nomor resi. J&T dan JNE bahkan meminta 4–5 digit terakhir HP penerima.
- **Fase A:** tombol "Salin resi" + link ke halaman lacak kurir.
- **Fase B:** API pelacakan banyak kurir. Pilihannya:
  - **BinderByte:** 28 kurir, kuota prabayar, polling.
  - **Biteship:** Rp10 per request, polling untuk resi dari luar Biteship.
  - **Komerce/RajaOngkir.**

### 2.1b Keputusan Pilot (2026-11-11)
- **Batas kirim:** hari kerja berikutnya, 23.59 WIB, setelah lunas (model "Jasa Kirim Toko" Shopee).
- **Hari kerja:** Senin–Sabtu. Minggu & libur nasional tidak dihitung; daftar libur lewat env.
- **Selesai:**
  - Pembeli bisa menekan "Pesanan Diterima".
  - Kalau tidak ditekan, selesai otomatis 2 hari setelah ditandai tiba, atau 7 hari setelah dikirim. Jumlah hari bisa diatur lewat env.
- **Batal setelah bayar:** hanya admin. Refund manual di Dashboard Xendit, tercatat "refund perlu diproses".
- **REA:** terpasang resmi (`rea-agents` 6.7.0, morluto/rea). Hanya untuk observasi tampilan Seller Centre milik Pilot (2.0). **Tidak** dipakai untuk merekam/memakai API privat Shopee: API pelacakan Shopee hanya mengenal pesanan Shopee, bukan resi pesanan website.
- **Pelacakan resi otomatis (rekomendasi):** dibangun **sekarang** di tugas 2 lewat API cek resi BinderByte (lihat 2.6b). Fitur ini **mati** sampai `BINDERBYTE_API_KEY` diisi, jadi tidak menunggu akun.

### 2.2 Rancangan status untuk Station Parfume
Pesanan dikirim manual dari toko dan ongkir ditanggung toko. Status lama dipertahankan, ditambah **satu** status baru, `DELIVERED` (setara Shopee/Tokopedia "Pesanan Tiba"):

```
PENDING_PAYMENT ─(lunas via Xendit)─▶ PAID ─(admin "Kemas")─▶ PROCESSING ─(admin "Kirim": kurir + resi)─▶ SHIPPED
      │                                 │                         │                                         │
      │ pembeli/admin batal             │ admin batal             │ admin batal                             ├─(admin "Tiba")─▶ DELIVERED
      │ atau lewat 24 jam               ▼                         ▼                                         │                  │
      ▼                          CANCELLED + refund "perlu diproses" → admin tandai refund selesai           └──────┬───────────┘
CANCELLED / EXPIRED                                                                          pembeli "Pesanan Diterima"
                                                                                             atau otomatis setelah N hari
                                                                                                        ▼
                                                                                                    COMPLETED
SHIPPED / DELIVERED / COMPLETED ─(owner/admin "Pengembalian dana", kasus khusus)─▶ REFUNDED
```

**Aturan yang dipilih Pilot (2026-11-11)** — semua bisa diubah lewat env tanpa ubah kode:

| Aturan | Nilai | Env (default) |
|---|---|---|
| Batas kirim (`ship_by`) | **Hari kerja berikutnya** setelah lunas, jam 23.59 WIB (aturan "Jasa Kirim Toko" Shopee) | `ORDER_SHIP_DAYS=1` |
| Hari kerja | **Senin–Sabtu**. Minggu & tanggal libur nasional tidak dihitung | `ORDER_WORK_DAYS=mon-sat`, `ORDER_HOLIDAYS=2026-12-25,…` |
| Zona waktu toko | WIB | `SHOP_TIMEZONE=Asia/Jakarta` |
| Pesanan selesai | Pembeli bisa menekan **"Pesanan Diterima"**. Kalau tidak, selesai **otomatis 2 hari setelah ditandai tiba**, atau **7 hari setelah dikirim** kalau tidak pernah ditandai tiba | `ORDER_AUTO_COMPLETE_AFTER_DELIVERED_DAYS=2`, `ORDER_AUTO_COMPLETE_AFTER_SHIPPED_DAYS=7` |
| Batal setelah bayar | **Hanya admin** (owner/admin). Pembeli menghubungi toko | — |

Contoh batas kirim:
- Lunas Senin 10.00 → kirim paling lambat Selasa 23.59.
- Lunas Sabtu → Senin 23.59 (Minggu dilewati).
- Lunas Jumat sebelum libur Senin → Selasa 23.59.

Aturan baru:
- **`PAID` hanya dari Xendit.** Admin tidak bisa lagi menandai lunas manual. Sekarang bisa, dan itu berisiko membuat pembayaran ganda.
- **Pembatalan oleh admin juga mematikan link bayar Xendit.** Sekarang link tetap hidup, jadi pembeli masih bisa membayar order yang sudah dibatalkan.
- **Setiap perubahan mencatat pelaku dan waktunya** (lihat 2.3).

| Status | Tab admin (ala Shopee) | Tab pembeli | Label pembeli |
|---|---|---|---|
| PENDING_PAYMENT | Belum Bayar | Belum Bayar | Menunggu pembayaran |
| PAID | Perlu Dikirim → Perlu Diproses | Dikemas | Pembayaran diterima |
| PROCESSING | Perlu Dikirim → Telah Diproses | Dikemas | Pesanan sedang dikemas |
| SHIPPED | Dikirim | Dikirim | Pesanan dikirim |
| DELIVERED | Dikirim → Sudah Sampai | Dikirim | Pesanan tiba |
| COMPLETED | Selesai | Selesai | Pesanan selesai |
| CANCELLED / EXPIRED | Pembatalan | Dibatalkan | Dibatalkan / Dibatalkan otomatis |
| REFUNDED | Pengembalian | Dibatalkan | Dana dikembalikan |

### 2.3 Perubahan database — migrasi baru `migrations/00002_order_tracking.sql`
Migrasi `00001` tidak diubah, karena sudah ada di master.

`orders`, kolom baru:
- `processed_at`: waktu dikemas.
- `ship_by`: batas kirim, = waktu lunas + masa pengemasan.
- `courier`, `courier_service`, `tracking_number`: kurir & resi.
- `shipping_cost`: biaya kirim yang dibayar toko ke kurir; opsional, untuk laporan ongkir yang ditanggung toko.
- `delivered_at`: waktu tiba. Foto bukti terima (link CDN, opsional) disimpan di metadata event "tiba".
- `auto_complete_at`: kapan otomatis selesai.
- `cancelled_by`: `buyer` | `admin` | `system`.
- `refund_status`: `pending` | `done`, plus `refund_amount` dan `refunded_at`.
- `seller_note`, `seller_note_updated_at`: catatan internal penjual, setara "catatan" di Shopee.
- `tracking_status`, `tracking_checked_at`: status terakhir dari kurir & kapan terakhir dicek (2.6b).

`orders`, constraint & index:
- CHECK status ditambah `DELIVERED`.
- Index `(status, ship_by)` untuk pesanan yang perlu dikirim / terlambat.
- Index `(auto_complete_at)`.
- Index `tracking_number` untuk pencarian resi.

Tabel baru `order_events`: riwayat + timeline, satu baris per kejadian.
- **Kolom:**
  - `order_id`
  - `type`, dengan CHECK daftar tetap: created, paid, processing, shipped, tracking_updated, delivered, completed, cancelled, expired, refund_pending, refunded, late_payment, note_updated, payment_failed.
  - `status_from`, `status_to`
  - `actor`: `buyer` | `staff` | `system` | `xendit`
  - `actor_user_id`
  - `title`: teks tampilan, mis. "Pesanan dikirim".
  - `description`: mis. "JNE REG · resi JP1234…".
  - `is_public`: tampil ke pembeli atau hanya admin.
  - `metadata` (JSONB)
  - `created_at`
- **Index:** `(order_id, created_at)` untuk timeline, dan `(type, created_at)` untuk metrik.
- **Isi awal (backfill):** event dibuat dari timestamp order yang sudah ada.

Tabel ini sekaligus mengembalikan **jejak audit khusus pesanan** (siapa melakukan apa, kapan) tanpa menghidupkan lagi audit log umum yang sudah dihapus.

### 2.4 API admin — tiruan "Pesanan Saya" di Shopee Seller Centre
Semua di `/api/v1/admin/orders`.

Hak akses:
- **owner / admin:** semua aksi.
- **warehouse:** kemas, kirim, ubah resi, tandai tiba, catatan.
- **cs:** hanya melihat + catatan. Ini sekaligus memberi peran `cs` endpoint pertamanya.

| Endpoint | Fungsi |
|---|---|
| `GET /admin/orders?tab=&q=&from=&to=&late=&refund=&courier=&sort=&page=&per_page=` | Daftar pesanan per tab (semua, belum_bayar, perlu_diproses, telah_diproses, dikirim, sudah_sampai, selesai, pembatalan, pengembalian). `q` mencari no. pesanan, nama/HP penerima, nama/email pembeli, nama produk, no. resi. Tiap baris: no. pesanan, waktu, pembeli, produk pertama + foto, jumlah item, total, status + label, **batas kirim + tanda terlambat**, kurir/resi, kanal bayar, status refund |
| `GET /admin/orders/summary` | Penghitung "Yang Perlu Dilakukan": jumlah per tab, **perlu dikirim hari ini**, **terlambat dikirim**, belum bayar yang akan kedaluwarsa < 1 jam, sudah sampai menunggu konfirmasi, **refund perlu diproses** |
| `GET /admin/orders/:id-atau-no` | Detail lengkap: data pembeli (nama, email, HP), alamat, item (snapshot, berat), total + **total berat**, pembayaran (status, kanal, waktu lunas, ID Xendit), **timeline lengkap** termasuk event internal dan siapa pelakunya, catatan penjual, kurir/resi + link lacak, batas kirim / terlambat, otomatis selesai kapan, refund, riwayat stok terkait, dan **`allowed_actions`** (tombol apa yang boleh muncul untuk peran & status saat itu) |
| `POST /admin/orders/:id/process` | PAID → PROCESSING ("Kemas") |
| `POST /admin/orders/:id/ship` `{courier, courier_service?, tracking_number}` | PAID/PROCESSING → SHIPPED. `courier` dari daftar tetap: jne, jnt, sicepat, anteraja, ninja, pos, idexpress, lion, spx, gosend, grab, kurir_toko, lainnya. Resi wajib kecuali kurir_toko/instan |
| `PATCH /admin/orders/:id/shipping` | Ubah kurir/resi (salah input); tercatat sebagai "Resi diubah" |
| `POST /admin/orders/:id/deliver` | SHIPPED → DELIVERED ("Pesanan tiba"); hitung ulang `auto_complete_at` |
| `POST /admin/orders/:id/complete` | SHIPPED/DELIVERED → COMPLETED (owner/admin, mis. pembeli konfirmasi lewat WhatsApp) |
| `POST /admin/orders/:id/cancel` `{reason}` | Belum bayar → batal + link Xendit dimatikan. Sudah bayar (PAID/PROCESSING) → batal + **stok produk umum dikembalikan** (riwayat stok "return") + refund "perlu diproses". Alasan dari daftar ala Shopee: stok habis, permintaan pembeli, alamat tidak terjangkau, lainnya |
| `POST /admin/orders/:id/refund` `{amount?, note}` | Tandai refund selesai (dikerjakan manual di Dashboard Xendit). Pembayaran ditandai REFUNDED. Untuk SHIPPED/DELIVERED/COMPLETED → status REFUNDED |
| `PUT /admin/orders/:id/note` `{note}` | Catatan penjual (internal) |
| `GET /admin/orders/export.csv?…` | Ekspor CSV dengan filter yang sama (ala "Ekspor" Shopee) |
| `GET /admin/orders/metrics?from=&to=` | "Performa Toko" (owner/admin), lihat 2.6 |

`PUT /orders/:id/status` yang lama **dihapus**, diganti aksi-aksi eksplisit di atas. Setiap aksi punya validasi sendiri dan mencatat event.

### 2.5 API pembeli
- `GET /orders?tab=belum_bayar|dikemas|dikirim|selesai|dibatalkan&page=`: tab ala "Pesanan Saya".
- `GET /orders/:id-atau-no`: bisa pakai nomor pesanan, karena halaman sukses Xendit membawa `?order=SP…`. Isinya:
  - label status
  - **timeline publik**
  - kurir + resi + link lacak
  - batas otomatis selesai
  - `can_cancel` / `can_confirm_received`
  - **Data internal disembunyikan:** ID Xendit, `failure_reason` mentah, catatan penjual, event internal. Saat ini semua itu terlihat oleh pembeli.
- `POST /orders/:id/received`: tombol "Pesanan Diterima" (SHIPPED/DELIVERED → COMPLETED).
- `PUT /orders/:id/cancel` tetap, hanya untuk yang belum bayar; sekarang tercatat `cancelled_by = buyer` + event.

### 2.6 Otomatisasi & metrik
Job berjalan di loop rekonsiliasi yang sudah ada (dirapikan di tugas 3):
- **Otomatis selesai:** SHIPPED/DELIVERED yang lewat `auto_complete_at` → COMPLETED (pelaku `system`).
- **Kedaluwarsa belum bayar** (sudah ada): sekarang juga mencatat event + `cancelled_by = system`.
- **Terlambat kirim dihitung saat dibaca** (`ship_by` lewat dan belum dikirim), tanpa job. Pesanan **tidak** dibatalkan otomatis; itu merugikan toko sendiri.

Metrik `GET /admin/orders/metrics?from=&to=` (meniru "Performa Toko" Shopee). Periode default 7 hari terakhir, seperti Shopee; masa pengemasan memakai 30 hari:
- Pesanan dibuat / dibayar / selesai.
- Dibatalkan, dipisah per pelaku (pembeli / admin / sistem).
- Omzet, rata-rata nilai pesanan, konversi bayar.
- **Tingkat Keterlambatan Pengiriman** = dikirim setelah `ship_by` ÷ semua yang dikirim.
- **Tingkat Pesanan Tidak Terselesaikan** = dibatalkan admin setelah bayar + refund ÷ pesanan dibayar.
- **Rata-rata masa pengemasan** (lunas → dikirim), rata-rata sampai selesai.
- 10 produk terlaris.

### 2.6b Pelacakan resi otomatis (BinderByte)
Pengganti "tracking" Shopee untuk pesanan website. Paket dikirim manual lewat kurir biasa, jadi status perjalanannya diambil dari API cek resi banyak kurir.

- **Penyedia:** BinderByte, `GET https://api.binderbyte.com/v1/track?api_key=&courier=&awb=` ([dok](https://docs.binderbyte.com/docs/resi)).
  - 28 kurir, termasuk jne, jnt, sicepat, anteraja, ninja, pos, ide, lion, spx.
  - Respons berisi `summary.status` dan `history[]` (tanggal, keterangan, lokasi).
  - Penyedia di-bungkus interface `TrackingProvider`, jadi Biteship bisa menggantikannya nanti tanpa ubah logika pesanan.
- **Data yang dikirim ke BinderByte:** kode kurir + resi. Khusus JNE ditambah 5 digit terakhir HP penerima (diambil dari `recipient_phone`, sudah tersimpan). Tidak ada data lain.
- **Mati otomatis** kalau `BINDERBYTE_API_KEY` kosong. Admin tetap bisa menandai "Pesanan tiba" manual.
- **Job polling** (ikut loop latar belakang):
  - Hanya pesanan `SHIPPED`/`DELIVERED` yang punya resi dan kurirnya didukung.
  - Interval default 3 jam (`TRACKING_POLL_INTERVAL`). Maksimal N resi per putaran (`TRACKING_MAX_PER_RUN=50`) supaya kuota tidak boros.
  - Berhenti mengecek setelah kurir melaporkan terkirim, atau 14 hari setelah dikirim.
  - Error/kuota habis hanya dicatat di log; pesanan tidak berubah.
- **Hasil:**
  - Setiap baris riwayat kurir yang baru → `order_events` tipe `tracking_updated`, **tampil ke pembeli** ("Paket tiba di gudang Jakarta", lokasi, waktu). Tidak dobel: setiap baris diberi sidik unik (waktu + keterangan).
  - Kurir melaporkan terkirim → status otomatis `SHIPPED → DELIVERED` (pelaku `system`), lalu otomatis selesai 2 hari kemudian sesuai keputusan Pilot.
  - Kolom baru di `orders`: `tracking_status` (status terakhir dari kurir) dan `tracking_checked_at`.
- **Tombol admin "Cek resi sekarang":** `POST /admin/orders/:id/tracking/refresh`, dibatasi 1× per 10 menit per pesanan.
- **Tab admin baru di "Dikirim":** "Paket bermasalah", yaitu resi yang tidak ditemukan kurir setelah 2 hari atau berstatus retur/gagal kirim.
- **Untuk Pilot nanti:** daftar akun di binderbyte.com, beli kuota, isi `BINDERBYTE_API_KEY` di `.env`. Bisa kapan saja; tidak menghalangi pekerjaan.

### 2.7 Cek: apakah detail isi pesanan sudah cukup?
**Jawaban: belum.** Ada dua masalah:
- Admin sama sekali **tidak bisa membuka** detail pesanan orang lain.
- Banyak field "Rincian Pesanan" Shopee yang belum ada.

Sebaliknya, pembeli justru melihat terlalu banyak data internal. Perbandingan dengan `get_order_detail` Shopee:

| Field Shopee | Sekarang | Setelah rencana ini |
|---|---|---|
| `order_sn`, `order_status`, waktu dibuat/diubah | ✅ `order_no`, status | ✅ + label status + `DELIVERED` |
| `message_to_seller` | ✅ `buyer_note` | ✅ |
| `days_to_ship`, `ship_by_date` | ❌ | ✅ `ship_by` + hitungan mundur + terlambat N hari |
| `buyer_user_id`, `buyer_username` | ⚠️ hanya `user_id`, nama/email tidak pernah dimuat | ✅ nama, email, HP pembeli di detail admin |
| `recipient_address` (nama, HP, kecamatan, kota, provinsi, kode pos, alamat lengkap) | ✅ snapshot | ✅ (kelurahan cukup di teks alamat, sama seperti form Shopee) |
| `note`, `note_update_time` (catatan penjual) | ❌ | ✅ `seller_note` + waktu |
| `pay_time` | ✅ `paid_at` | ✅ |
| `payment_method` | ❌ kolom ada, **tidak pernah diisi** | ✅ diisi dari Xendit `channel_code` (tugas 4) |
| `total_amount`, ongkir | ✅ (ongkir 0) | ✅ + **biaya kirim yang dibayar toko** (opsional, untuk laporan biaya ongkir yang ditanggung toko) |
| `shipping_carrier`, nomor resi | ❌ | ✅ `courier`, `courier_service`, `tracking_number` + link lacak |
| `pickup_done_time` | ✅ `shipped_at` | ✅ + `processed_at`, `delivered_at` (+ foto bukti terima opsional) |
| `order_chargeable_weight_gram` | ⚠️ berat per item saja | ✅ total berat dihitung (untuk kirim manual) |
| `cancel_by`, `cancel_reason`, `buyer_cancel_reason` | ⚠️ alasan teks saja; tidak tahu siapa yang membatalkan | ✅ `cancelled_by` + kode alasan + teks |
| `item_list` (nama, varian, SKU, qty, harga, foto, berat) | ✅ snapshot lengkap (+ tarif per ml, harga botol) | ✅ |
| Riwayat pesanan ("Periksa Rincian") / `tracking_info[]` | ❌ | ✅ timeline `order_events` (publik untuk pembeli, lengkap untuk admin) |
| Riwayat transaksi | ⚠️ ada di `payments` tapi tidak bisa dilihat admin | ✅ di detail admin |
| `package_list`, partial cancel, COD, dropshipper, invoice/pajak, `return_request_due_date` | ❌ | ⏭ tidak relevan sekarang (1 pesanan = 1 paket, tanpa COD) / fase B (retur) |

**Kebocoran yang ditutup:** respons pesanan untuk pembeli saat ini berisi objek `payments` mentah, yaitu ID Xendit dan `failure_reason` internal (mis. "pembayaran ganda … refund manual"). Setelah perbaikan, pembeli hanya melihat status, nominal, kanal, waktu bayar, dan link bayar selama masih aktif.

### 2.8 Fase B (nanti, arahnya sudah disiapkan)
1. ~~Pelacakan kurir otomatis~~ → dipindah ke tugas 2 (2.6b).
2. **Sinkron pesanan dari toko Shopee** (kalau dibutuhkan): lewat Shopee Open Platform resmi (daftar developer + otorisasi toko), bukan API privat.
3. **Pengajuan batal dari pembeli** untuk pesanan yang sudah dibayar, disetujui/ditolak admin dalam 24 jam.
3. **Pengembalian barang/dana & pengiriman gagal** (paket kembali ke toko), refund lewat API Xendit.
4. **Notifikasi** email/WhatsApp setiap event.
5. **Aksi massal**, cetak label/packing slip/invoice.
6. **Ongkir otomatis.**

## 3. Rincian implementasi (kalau disetujui)

Fungsi yang dipakai ulang:
- `catalog.SetStock` (pola `stock_movements` + `user_id`): pengembalian stok saat batal setelah bayar memakai pola yang sama dengan `commitStock`/`releaseStock` di `src/modules/order/services/order.services.go`.
- `paymentservices.CancelOrderSessions`: sudah dipakai saat pembeli membatalkan; sekarang juga dipanggil dari controller pembatalan admin. Pemanggilan di controller menghindari import cycle, karena payment sudah meng-import order.
- `authmiddlewares.RoleBasedAccess` per route.
- `staffID(c)` (pola di `products/controller/admin.controller.go`).
- `lockOrder` (`SELECT … FOR UPDATE`).

File baru:
- `migrations/00002_order_tracking.sql`:
  - kolom baru `orders`
  - CHECK `DELIVERED` + `cancelled_by` + `refund_status`
  - tabel `order_events`
  - index
  - backfill event dari timestamp lama
  - bagian Down (DELIVERED → SHIPPED, hapus tabel & kolom)
- `src/modules/order/services/transition.go`: `Transition(tx, order, to, Actor, Opts)` menggantikan `ApplyStatus` + `ApplyLatePayment`.
  - Satu tabel aturan menentukan dari → ke, siapa yang boleh (buyer / staff + peran / system / xendit), timestamp yang diisi, efek stok (commit / release / return), `ship_by` / `auto_complete_at`, lalu menulis `order_events`.
  - Pembayaran telat (EXPIRED/CANCELLED → PAID) masuk tabel ini khusus pelaku `xendit`, jadi tidak lagi melewati aturan.
- `src/modules/order/services/sla.go`: hitung batas kirim (hari kerja, libur, zona waktu) & otomatis selesai; konfigurasi dari env.
- `src/modules/order/services/admin.services.go`: daftar (tab / filter / cari / urut / halaman), ringkasan, detail + `allowed_actions`, aksi, ekspor CSV, metrik (agregasi SQL `count(*) FILTER (…)`).
- `src/modules/order/services/jobs.go`: `AutoCompleteDue(now)`.
- `src/modules/order/couriers.go`: daftar kode kurir, nama, link halaman lacak, wajib resi atau tidak, kode BinderByte.
- `src/modules/order/tracking/`: interface `TrackingProvider`, klien BinderByte (timeout, batas ukuran respons, API key tidak pernah masuk log), job `PollShipments`, tombol "Cek resi sekarang".
- `src/modules/order/models/`: DTO pembeli (tanpa data internal) & admin, request aksi.
- `src/modules/order/controller/admin.controller.go`.

File diubah:
- `src/repository/order.go`: field baru, struct `OrderEvent`, status `DELIVERED`, label & tab.
- `order.services.go`:
  - Checkout mencatat event "dibuat".
  - Pembatalan pembeli: pelaku `buyer`.
  - Kedaluwarsa: pelaku `system`.
  - Daftar pesanan pembeli dengan tab + halaman.
  - Detail bisa pakai id atau nomor pesanan.
  - Tombol "Pesanan Diterima".
- `order.route.go`:
  - Rute pembeli.
  - Grup `/admin/orders`.
  - **Hapus `PUT /orders/:id/status`.**
- `payment.services.go`:
  - Lunas lewat `Transition` dengan pelaku `xendit`.
  - Rekonsiliasi dengan pelaku `system`.
  - Event internal saat pembayaran gagal.
- `main.go`: job otomatis selesai ikut loop latar belakang.
- `.env.example`: env `ORDER_*`, `SHOP_TIMEZONE`, `BINDERBYTE_API_KEY`, `TRACKING_POLL_INTERVAL`, `TRACKING_MAX_PER_RUN`.
- `docs/schema/README.md`, `docs/audit-and-roadmap.md`.
