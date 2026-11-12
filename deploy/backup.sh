#!/usr/bin/env bash
# Backup database production → deploy/backups/station-parfume-<waktu>.sql.gz,
# disimpan KEEP_DAYS hari (default 14). Contoh cron harian jam 03.00:
#   0 3 * * * /home/deploy/Station-parfume-backend/deploy/backup.sh >> /home/deploy/backup.log 2>&1
# Salin juga folder backups ke luar VPS (mis. rclone ke cloud storage).
# Restore ke database KOSONG: lihat docs/deploy-vps.md.
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
KEEP_DAYS="${KEEP_DAYS:-14}"
OUT="${BACKUP_DIR:-$DIR/backups}"
mkdir -p "$OUT"
chmod 700 "$OUT"
file="$OUT/station-parfume-$(date +%Y%m%d-%H%M%S).sql.gz"
trap 'rm -f "$file.tmp"' EXIT
"$DIR/compose.sh" exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-privileges' | gzip -9 > "$file.tmp"
mv "$file.tmp" "$file"
chmod 600 "$file"
find "$OUT" -name 'station-parfume-*.sql.gz' -mtime +"$KEEP_DAYS" -delete
echo "$(date '+%F %T') backup OK: $file ($(du -h "$file" | cut -f1))"
