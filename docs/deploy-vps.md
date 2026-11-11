# Deploy ke VPS sendiri

Panduan langkah demi langkah untuk menjalankan backend Station Parfume di satu VPS
dengan Docker Compose dan HTTPS otomatis. Semua perintah di sini sudah diuji lokal
(lihat [Hasil uji](#hasil-uji-lokal-2026-11-11)).

> **Pembayaran live** baru bisa setelah aktivasi akun Xendit disetujui. Sebelum itu
> server tetap bisa di-deploy dengan key TEST Xendit, atau tanpa key sama sekali
> (endpoint bayar & webhook menjawab 503).

## Ringkasan perintah

Semua dijalankan dari folder repo di VPS (`~/Station-parfume-backend`).

| Perintah | Fungsi |
|---|---|
| `deploy/update.sh` | Deploy pertama **dan** setiap update: backup → build → migrasi → ganti api |
| `deploy/compose.sh ps` | Status semua container |
| `deploy/compose.sh logs -f --tail 100 api` | Log api (ganti `api` dengan `caddy`, `db`, `redis`, `migrate`) |
| `deploy/compose.sh up -d` | Terapkan perubahan `deploy/.env.prod` |
| `deploy/backup.sh` | Backup database sekarang |

## Gambaran

```
Internet ──80/443 (IPv4)──▶ caddy ──▶ api:8080 ──▶ db    (Postgres 17, volume db-data)
                            HTTPS otomatis    └──▶ redis (Redis 7, password, AOF, volume redis-data)
                            Let's Encrypt
migrate: jalan sekali setiap deploy, sebelum api start (image yang sama dengan api)
```

- Satu VPS, semua service di Docker Compose (`deploy/docker-compose.prod.yml`).
- **Hanya Caddy** yang membuka port ke internet. api, db, dan redis hanya ada di jaringan
  internal Docker `172.30.10.0/24`.
- Container api & migrate: user non-root, filesystem read-only, tanpa capability Linux.

| File | Isi |
|---|---|
| `deploy/docker-compose.prod.yml` | Definisi service caddy, api, migrate, db, redis |
| `deploy/Caddyfile` | `API_DOMAIN` → `api:8080`, HTTPS otomatis, kompresi |
| `deploy/.env.prod.example` | Contoh env production. Salinannya `deploy/.env.prod` berisi secret (di `.gitignore`) |
| `deploy/update.sh` | Deploy/update aman |
| `deploy/compose.sh` | `docker compose` dengan file & env production |
| `deploy/backup.sh` | `pg_dump` terkompresi ke `deploy/backups/`, disimpan 14 hari |

## 1. Kebutuhan

- **VPS** Ubuntu 24.04 LTS, 1–2 vCPU, **2 GB RAM**, SSD 25–40 GB. Lokasi Jakarta atau
  Singapura supaya dekat dengan pembeli.
  - Image dibangun di VPS. Build Go butuh ±1 GB RAM, jadi untuk RAM ≤ 2 GB tambahkan swap
    (langkah 2).
- **Domain/subdomain untuk API**, mis. `api.tokoanda.com`, plus akses ke pengaturan DNS-nya.
- Akses repo GitHub dari VPS (deploy key read-only, langkah 4).

## 2. Siapkan VPS (sekali)

**User non-root + login SSH key.** Login sebagai root dari panel penyedia, lalu:

```bash
adduser deploy                 # buat password sementara
usermod -aG sudo deploy
```

Salin SSH key ke user `deploy`:
- Kalau root sudah login dengan SSH key: `rsync --archive --chown=deploy:deploy ~/.ssh /home/deploy` (di VPS, sebagai root).
- Kalau belum: `ssh-copy-id deploy@IP_VPS` dari laptop.

Lalu coba login sebagai `deploy` di terminal baru: `ssh deploy@IP_VPS`.

Setelah login dengan key **berhasil**, matikan login password & root. Nama file `00-`
supaya menang dari `50-cloud-init.conf` bawaan penyedia (sshd memakai nilai pertama
yang ditemukan).

```bash
sudo tee /etc/ssh/sshd_config.d/00-station-parfume.conf >/dev/null <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
EOF
sudo sshd -t && sudo systemctl restart ssh
```

Jangan tutup sesi SSH lama sebelum login baru terbukti berhasil.

**Update otomatis, zona waktu, firewall:**

```bash
sudo apt update && sudo apt full-upgrade -y
sudo apt install -y unattended-upgrades && sudo dpkg-reconfigure -plow unattended-upgrades
sudo timedatectl set-timezone Asia/Jakarta    # jadwal cron backup memakai WIB

sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 443/udp                        # HTTP/3
sudo ufw enable
```

> **Penting:** port yang di-publish Docker **melewati aturan ufw**. Karena itu di compose
> hanya Caddy yang punya `ports:`. **Jangan pernah menambahkan `ports:` ke db, redis, atau
> api**, karena database/redis langsung terbuka ke internet.

**Docker Engine + Compose:** ikuti panduan resmi
<https://docs.docker.com/engine/install/ubuntu/>, bagian *Install using the apt repository*.
Lalu:

```bash
sudo usermod -aG docker deploy    # logout lalu login lagi
docker compose version            # cek terpasang
```

Anggota grup `docker` setara root. Jadi hanya user `deploy` yang dimasukkan, dan login
SSH-nya hanya pakai key.

**Swap 2 GB** (kalau RAM ≤ 2 GB):

```bash
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

## 3. DNS

- Buat record **A**: `api.tokoanda.com` → IP publik (IPv4) VPS.
- **Jangan buat record AAAA (IPv6).**
  - Caddy sengaja hanya dibuka di IPv4.
  - Lewat IPv6, Docker meneruskan koneksi melalui proxy-nya sendiri. Akibatnya semua
    pengunjung IPv6 terlihat ber-IP sama (`172.30.10.1`) dan berbagi batas rate limit login
    (terbukti saat uji).
  - Pengunjung dari jaringan IPv6 tetap bisa masuk lewat IPv4.
- Cek dengan `dig +short api.tokoanda.com`: hasilnya harus IP VPS. Tunggu sampai benar
  sebelum langkah 5, karena Caddy butuh DNS ini untuk mengambil sertifikat HTTPS.

## 4. Ambil kode & isi env

**Deploy key read-only** (VPS hanya bisa membaca repo, tidak bisa push):

```bash
ssh-keygen -t ed25519 -f ~/.ssh/station_parfume_deploy -N '' -C deploy-vps
cat ~/.ssh/station_parfume_deploy.pub
```

1. Tempel isi `.pub` di GitHub → repo → *Settings* → *Deploy keys* → *Add deploy key*.
   Biarkan *Allow write access* **tidak** dicentang.
2. Pakai key itu untuk github.com dan clone repo:

```bash
cat >> ~/.ssh/config <<'EOF'
Host github.com
  IdentityFile ~/.ssh/station_parfume_deploy
  IdentitiesOnly yes
EOF
chmod 600 ~/.ssh/config
git clone git@github.com:<akun>/<repo>.git ~/Station-parfume-backend
cd ~/Station-parfume-backend
```

**Isi env production.** Empat secret dibuat acak langsung ke file, tanpa tampil di layar:

```bash
cp deploy/.env.prod.example deploy/.env.prod
chmod 600 deploy/.env.prod
for v in DB_PASSWORD REDIS_PASSWORD JWT_SECRET JWT_REFRESH_SECRET; do
  sed -i "s|^$v=.*|$v=$(openssl rand -hex 32)|" deploy/.env.prod
done
nano deploy/.env.prod
```

Yang wajib diganti di `nano`:

| Variabel | Isi |
|---|---|
| `API_DOMAIN` | `api.tokoanda.com` (tanpa `https://`) |
| `FRONTEND_URL` | `https://tokoanda.com` |
| `CORS_ALLOWED_ORIGINS` | Semua domain frontend, mis. `https://tokoanda.com,https://www.tokoanda.com` |
| `GOOGLE_REDIRECT_URL` | `https://api.tokoanda.com/api/v1/auth/google/callback` |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Dari Google Cloud Console (boleh kosong dulu) |
| `XENDIT_SECRET_KEY` / `XENDIT_WEBHOOK_TOKEN` | Dari Dashboard Xendit (boleh kosong dulu) |
| `CHECKOUT_SUCCESS_URL` / `CHECKOUT_CANCEL_URL` | Halaman frontend setelah bayar/batal (HTTPS) |

Aturan file: tanpa `$`, spasi, atau tanda kutip di nilai, dan komentar di baris sendiri.

> `deploy/.env.prod` dan `deploy/backups/` tidak ikut git. **Jangan jalankan
> `git clean -x`** di VPS, karena perintah itu menghapus keduanya.

## 5. Jalankan pertama kali

```bash
deploy/update.sh
```

Build pertama ±2–5 menit. Di akhir, `ps` harus menunjukkan `api`, `db`, `redis`
**healthy** dan `caddy` **Up**. Lalu:

```bash
curl https://api.tokoanda.com/readyz
# {"status":"ok","checks":{"database":"ok","redis":"ok"}}
```

Swagger `/docs` sengaja mati di production (404).

## 6. Isi data awal

**Katalog** dari CSV yang ada di dalam image. Jalankan sekali di awal. Menjalankan ulang
aman, tapi kolom CSV yang **terisi** menimpa perubahan yang dibuat admin lewat website;
kolom kosong tidak mengubah apa-apa (lihat `seed/README.md`).

```bash
deploy/compose.sh run --rm --no-deps --entrypoint /app/bin/seed api -dry-run   # cek dulu, tidak menyimpan
deploy/compose.sh run --rm --no-deps --entrypoint /app/bin/seed api            # simpan
```

**Akun owner.** Password diketik dan tidak tersimpan di file maupun riwayat shell:

```bash
read -rp 'Nama owner: ' ADMIN_NAME; read -rp 'Email owner: ' ADMIN_EMAIL
read -rsp 'Password owner (min 8): ' ADMIN_PASSWORD; echo
export ADMIN_NAME ADMIN_EMAIL ADMIN_PASSWORD
deploy/compose.sh run --rm --no-deps --entrypoint /app/bin/seed -e ADMIN_NAME -e ADMIN_EMAIL -e ADMIN_PASSWORD api -admin
unset ADMIN_PASSWORD
```

Output `owner: dibuat`. Login staf di `POST https://api.tokoanda.com/api/v1/auth/login/admin`.

## 7. Sambungkan layanan luar

| Layanan | Yang diisi |
|---|---|
| Frontend | Base URL API `https://api.tokoanda.com/api/v1`. Domain frontend harus ada di `CORS_ALLOWED_ORIGINS`, persis termasuk `https://` dan `www` |
| Google Cloud Console → Credentials → OAuth client | *Authorized redirect URIs* = nilai `GOOGLE_REDIRECT_URL`, persis |
| Dashboard Xendit → Settings → Webhooks | URL webhook Payment Sessions: `https://api.tokoanda.com/api/v1/webhooks/xendit` (event `payment_session.completed` & `payment_session.expired`). Salin *verification token* ke `XENDIT_WEBHOOK_TOKEN`. Mode TEST dan LIVE punya pengaturan masing-masing |

Setiap selesai mengubah `deploy/.env.prod`:

```bash
deploy/compose.sh up -d     # container yang env-nya berubah dibuat ulang
```

## 8. Update ke versi baru

```bash
cd ~/Station-parfume-backend
git pull
deploy/update.sh
curl -fsS https://api.tokoanda.com/readyz
```

`update.sh` menjalankan langkah berikut berurutan:
1. Backup database.
2. Build image baru.
3. Migrasi database dengan image baru.
4. Mengganti container api.

**Kalau langkah 1–3 gagal, api yang sedang jalan tidak disentuh** (terbukti dengan migrasi
rusak saat uji). Saat api diganti, situs tidak bisa diakses beberapa detik (Caddy menjawab
502), karena hanya ada satu container api.

> Jangan update dengan `deploy/compose.sh up -d --build`. Compose membuat ulang api
> **sebelum** menunggu migrasi, jadi kalau migrasi gagal api ikut mati (terbukti saat uji).

**Kalau `update.sh` berhenti di langkah migrasi:**
- api lama tetap melayani.
- Perbaiki migrasinya, atau kembali ke versi sebelumnya (bagian 9).
- Sampai itu beres, jangan jalankan `deploy/compose.sh up -d`. Tag image `latest` sudah
  menunjuk ke versi baru yang gagal, jadi `up -d` akan mencoba memakainya.

## 9. Rollback (kembali ke versi sebelumnya)

```bash
git log --oneline -10            # cari commit yang mau dipakai
git checkout <commit>
deploy/update.sh
```

- **Kode lama tetap jalan dengan database yang sudah dimigrasi lebih baru.** Server mencatat
  `PERINGATAN schema`, dan `migrate up` tidak melakukan apa-apa (terbukti saat uji). Ini
  aman selama migrasi baru hanya *menambah* (tabel/kolom baru).
- Kalau migrasi baru *menghapus atau mengganti nama* kolom, ada dua pilihan:
  - batalkan migrasinya **sebelum** checkout memakai image versi baru:
    `deploy/compose.sh run --rm migrate down -yes` (satu migrasi per perintah; bisa
    menghapus data);
  - atau pulihkan backup yang otomatis dibuat `update.sh` sebelum update (bagian 10).
    Data sejak update hilang.
- Kembali ke versi terbaru: `git checkout master && git pull && deploy/update.sh`.

## 10. Backup & restore

**Backup** ke `deploy/backups/station-parfume-YYYYMMDD-HHMMSS.sql.gz`:
- File berizin 600 dan folder 700.
- Backup yang lebih tua dari 14 hari dihapus otomatis. Ubah dengan `KEEP_DAYS=30 deploy/backup.sh`.

Jadwalkan harian jam 03.00 (`crontab -e`):

```
0 3 * * * /home/deploy/Station-parfume-backend/deploy/backup.sh >> /home/deploy/backup.log 2>&1
```

**Wajib salin backup ke luar VPS.** Kalau VPS rusak atau hilang, backup di dalamnya ikut
hilang. Pilihannya:
- unduh berkala ke laptop:
  `rsync -av deploy@IP_VPS:Station-parfume-backend/deploy/backups/ ./backup-station-parfume/`
- atau pakai `rclone` ke Google Drive/S3/R2 dari cron.

**Latihan restore (bulanan).** Restore ke database terpisah, jadi production tidak
terganggu:

```bash
F=$(ls -t deploy/backups/station-parfume-*.sql.gz | head -1)
gunzip -c "$F" | deploy/compose.sh exec -T db sh -c 'createdb -U "$POSTGRES_USER" restore_test && psql -v ON_ERROR_STOP=1 -q -o /dev/null -U "$POSTGRES_USER" -d restore_test'
deploy/compose.sh exec -T db sh -c 'psql -U "$POSTGRES_USER" -d restore_test -c "select count(*) from orders"'
deploy/compose.sh exec -T db sh -c 'dropdb -U "$POSTGRES_USER" restore_test'
```

**Pemulihan production dari backup.** Semua data sejak waktu backup **hilang**.

```bash
F=deploy/backups/station-parfume-YYYYMMDD-HHMMSS.sql.gz      # pilih file
deploy/compose.sh stop api
deploy/compose.sh exec -T db sh -c 'dropdb -U "$POSTGRES_USER" "$POSTGRES_DB" && createdb -U "$POSTGRES_USER" "$POSTGRES_DB"'
gunzip -c "$F" | deploy/compose.sh exec -T db sh -c 'psql -v ON_ERROR_STOP=1 -q -o /dev/null -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
# ganti secret JWT: semua orang login ulang
for v in JWT_SECRET JWT_REFRESH_SECRET; do sed -i "s|^$v=.*|$v=$(openssl rand -hex 32)|" deploy/.env.prod; done
deploy/compose.sh up -d
```

Kenapa secret JWT diganti: ID user/pesanan yang dibuat setelah backup akan terpakai ulang,
dan token lama jangan sampai berlaku untuk akun lain. Setelah pulih, cek Dashboard Xendit
untuk pembayaran sejak waktu backup. Pesanannya sudah tidak ada di database dan perlu
ditangani manual.

> **Jangan pernah `deploy/compose.sh down -v`** di production. `-v` menghapus volume
> database dan redis.

## 11. Log & pemantauan

- **Log:** `deploy/compose.sh logs -f --tail 100 api`, atau `--since 1h`.
  - Setiap container dibatasi 5 × 10 MB, jadi disk tidak penuh.
  - Log request berisi method, path (tanpa query string), status, durasi, IP pembeli, dan
    request ID.
  - Kalau frontend menampilkan header `X-Request-ID` saat error, lacak dengan
    `deploy/compose.sh logs api | grep <request-id>`.
  - Access log Caddy sengaja mati, karena URL bisa berisi kode login Google.
- **Uptime:** daftarkan `https://api.tokoanda.com/readyz` di UptimeRobot atau Better Stack
  (paket gratis), cek tiap 5 menit, dengan notifikasi email/Telegram.
  - `503`: database atau redis bermasalah (isi JSON menunjukkan yang mana).
  - Tidak bisa diakses: server atau Caddy mati.
- **Disk:** `df -h` dan `docker system df`. Image lama sisa update dibersihkan dengan
  `docker image prune -f`.

## 12. Perawatan rutin

| Kapan | Apa |
|---|---|
| Harian (otomatis) | Backup cron, `unattended-upgrades` |
| Setiap update | `git pull && deploy/update.sh`, lalu cek `/readyz` |
| Bulanan | Patch keamanan image: `deploy/compose.sh pull db redis caddy && deploy/compose.sh up -d`. Latihan restore. Salin backup ke luar VPS. `docker image prune -f` |

**Ganti secret:**
- **JWT:** ubah `JWT_SECRET`/`JWT_REFRESH_SECRET` lalu `deploy/compose.sh up -d`. Semua
  orang login ulang.
- **Redis:** ubah `REDIS_PASSWORD` lalu `up -d`.
- **Password database:** ganti dulu di Postgres
  (`deploy/compose.sh exec db psql -U <DB_USER> -d <DB_NAME> -c '\password <DB_USER>'`).
  Setelah itu isi nilai yang sama di `DB_PASSWORD`, lalu `up -d`. Mengubah env saja **tidak**
  mengganti password user yang sudah ada.

**Jangan:**
- naikkan versi mayor Postgres (17 → 18) dengan mengganti tag image. Data harus
  dipindah dengan dump/restore;
- `down -v`;
- tambahkan `ports:` ke db/redis/api;
- commit `deploy/.env.prod`.

## 13. Masalah umum

| Gejala | Penyebab & solusi |
|---|---|
| `update.sh` gagal di langkah 3 | Baca errornya; migrasi yang gagal sudah di-rollback otomatis. api lama tetap jalan. Lihat bagian 8 |
| Caddy tidak mendapat sertifikat | DNS A belum menunjuk ke VPS, atau port 80/443 tertutup (ufw / firewall panel penyedia). `deploy/compose.sh logs caddy` |
| Browser: error CORS, API menjawab 403 | Domain frontend belum ada di `CORS_ALLOWED_ORIGINS` (harus persis), lalu `up -d` |
| Login Google: `redirect_uri_mismatch` | `GOOGLE_REDIRECT_URL` tidak sama persis dengan di Google Cloud Console |
| Bayar / webhook menjawab 503 | Env Xendit kosong atau salah. Cari `PERINGATAN pembayaran` di log api |
| `api` unhealthy | `curl https://api.tokoanda.com/readyz` menunjukkan database/redis mana yang mati |
| Banyak pembeli kena 429 bersamaan | Semua terlihat ber-IP sama. Cek `TRUSTED_PROXIES=172.30.10.0/24` dan tidak ada record AAAA |

## Hasil uji lokal (2026-11-11)

Stack production dijalankan di mesin development dengan nama proyek terpisah
(`sp-prodtest`), port 18080/18443, `API_DOMAIN=localhost` (sertifikat lokal Caddy), dan
env hasil `.env.prod.example`. Setelah selesai, semuanya dihapus (`down -v`, image uji
dihapus).

| Cek | Hasil |
|---|---|
| Urutan start | db sehat → migrate `up 00001` exit 0 → redis sehat → api → caddy |
| `https://localhost:18443/readyz` | 200 `{"database":"ok","redis":"ok"}` lewat HTTP/2 |
| Port terbuka ke host | Hanya Caddy (80/443 tcp + 443 udp, IPv4). db, redis, api tidak punya port host |
| `/docs`, `/docs/openapi.yaml` | 404 (release) |
| HTTP → HTTPS | 308 |
| Header lewat Caddy | HSTS, `nosniff`, `X-Frame-Options`, CSP, `X-Request-ID`, `no-store`. Katalog dikompresi gzip |
| CORS | Origin asing 403, preflight origin terdaftar 204 |
| IP pembeli | Lewat IP LAN tercatat IP asli. `X-Forwarded-For` palsu diabaikan. Lewat IPv6 semua jadi `172.30.10.1`, karena itu Caddy dibatasi IPv4 dan sekarang IPv6 ditolak |
| Seed katalog & owner dari image | `-dry-run` OK, seed 587 varian, owner dibuat lewat `-e` tanpa file. Login owner 200 |
| Redis | Tanpa password ditolak (`NOAUTH`), AOF aktif. Token yang sudah logout tetap 401 setelah Redis restart |
| `server -healthcheck` | 0 saat sehat, 1 saat db dimatikan (`/readyz` 503), 0 lagi setelah db hidup |
| Hardening container | api: `nonroot`, read-only, `cap_drop ALL`, `no-new-privileges`. Caddy: hanya `NET_BIND_SERVICE` |
| `backup.sh` | Dump 16 tabel tanpa OWNER/GRANT, izin 600. Backup > 14 hari terhapus, file lain tidak |
| Restore ke DB kosong | Jumlah user/produk/varian & versi migrasi identik. `migrate status` OK |
| Pemulihan production | Data sebelum backup kembali, data sesudahnya hilang, api sehat |
| Rollback kode (DB lebih baru) | `migrate up` tidak melakukan apa-apa (exit 0), server jalan dengan peringatan |
| `update.sh` dengan migrasi rusak | Backup dibuat, migrasi di-rollback, skrip exit 1, **api lama tetap melayani** (container sama, `/readyz` 200) |
| `up -d --build` langsung dengan migrasi gagal | api dibuat ulang lalu tidak start (inilah alasan memakai `update.sh`) |
| Update tanpa perubahan | ±6 detik, api tidak dibuat ulang |
| `docker stop` api | Graceful shutdown, exit 0 |
