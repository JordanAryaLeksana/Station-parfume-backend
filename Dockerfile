# syntax=docker/dockerfile:1
# Image production Station Parfume API.
#   docker build -t station-parfume-api .
# Isi image akhir: hanya binary server, migrate, seed + CSV seed. Tanpa source
# code, tanpa .env, tanpa .git (lihat .dockerignore), tanpa shell, user non-root.

# ---------------------------------------------------------------- build
FROM golang:1.27.4-alpine AS build
WORKDIR /src
# Modul dulu: layer ini dipakai ulang selama go.mod/go.sum tidak berubah.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
RUN go build -ldflags="-s -w" -o /out/server . \
 && go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
 && go build -ldflags="-s -w" -o /out/seed ./seed \
 && mkdir -p /out/seed-data && cp seed/*.csv /out/seed-data/

# ---------------------------------------------------------------- runtime
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /out/migrate /out/seed /app/bin/
# CSV untuk: docker run --entrypoint /app/bin/seed <image>   (default -dir seed = /app/seed)
COPY --from=build /out/seed-data /app/seed
ENV GIN_MODE=release PORT=8080
EXPOSE 8080
USER nonroot:nonroot
# Migrasi dijalankan terpisah sebelum server: --entrypoint /app/bin/migrate <image> up
ENTRYPOINT ["/app/bin/server"]
