#!/usr/bin/env bash
# Deploy pertama DAN setiap update versi (setelah git pull / git checkout):
#   deploy/update.sh
# Urutan:
#   1. backup database (kalau db sudah jalan)
#   2. build image baru
#   3. migrasi database memakai image baru
#   4. baru setelah migrasi sukses, container api diganti versi baru
# Kalau langkah 1–3 gagal, skrip berhenti dan api yang sedang jalan TIDAK
# disentuh. Jangan update dengan `compose.sh up -d --build` langsung: compose
# membuat ulang api sebelum menunggu migrasi, jadi migrasi gagal = api mati.
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
C="$DIR/compose.sh"

if [ -n "$("$C" ps -q --status running db 2>/dev/null)" ]; then
  echo "==> 1/4 backup database"
  "$DIR/backup.sh"
else
  echo "==> 1/4 backup dilewati (db belum jalan — deploy pertama?)"
fi
echo "==> 2/4 build image"
"$C" build
echo "==> 3/4 migrasi database"
"$C" run --rm migrate
echo "==> 4/4 jalankan versi baru"
"$C" up -d --remove-orphans --wait --wait-timeout 180
"$C" ps
echo "Selesai. Cek: curl -fsS https://<API_DOMAIN>/readyz"
