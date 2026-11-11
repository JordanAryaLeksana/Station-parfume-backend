#!/usr/bin/env bash
# Test integrasi end-to-end: menyalakan PostgreSQL 17 + Redis 7 SEMENTARA di
# Docker (container sp-test-pg / sp-test-redis, port 55432 / 56379), menjalankan
# go test -tags=integration, lalu mematikan container. Database & Redis
# development tidak disentuh.
#
#   scripts/test-integration.sh              # semua test
#   scripts/test-integration.sh -run TestAuth -v
set -euo pipefail
cd "$(dirname "$0")/.."

PG=sp-test-pg
RD=sp-test-redis
PG_PORT=${TEST_PG_PORT:-55432}
RD_PORT=${TEST_REDIS_PORT:-56379}

cleanup() { docker rm -f "$PG" "$RD" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup

docker run -d --name "$PG" -e POSTGRES_PASSWORD=test -e POSTGRES_DB=sp_test -p "127.0.0.1:${PG_PORT}:5432" postgres:17 >/dev/null
docker run -d --name "$RD" -p "127.0.0.1:${RD_PORT}:6379" redis:7-alpine >/dev/null
for _ in $(seq 1 60); do
  docker exec "$PG" pg_isready -U postgres -d sp_test >/dev/null 2>&1 && break
  sleep 1
done
sleep 1

export DATABASE_URL="postgres://postgres:test@127.0.0.1:${PG_PORT}/sp_test?sslmode=disable"
export REDIS_ADDR="127.0.0.1:${RD_PORT}"
go test -tags=integration -count=1 ./tests/integration/... "$@"
