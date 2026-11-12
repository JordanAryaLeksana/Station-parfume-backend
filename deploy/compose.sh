#!/usr/bin/env bash
# Pembungkus docker compose production: memakai deploy/docker-compose.prod.yml
# dan deploy/.env.prod (atau ENV_FILE=/path/lain).
#   deploy/compose.sh ps                # status
#   deploy/compose.sh logs -f api       # log
#   deploy/compose.sh up -d             # terapkan perubahan .env.prod
# Untuk deploy/update versi pakai deploy/update.sh (migrasi dulu, baru ganti api).
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
export ENV_FILE="${ENV_FILE:-$DIR/.env.prod}"
if [ ! -f "$ENV_FILE" ]; then
  echo "File env tidak ada: $ENV_FILE (salin dari deploy/.env.prod.example)" >&2
  exit 1
fi
exec docker compose --env-file "$ENV_FILE" -f "$DIR/docker-compose.prod.yml" "$@"
