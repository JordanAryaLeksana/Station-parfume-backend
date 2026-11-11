# Gunakan image Go resmi
FROM golang:1.24.4

# Set working directory
WORKDIR /app

# Copy module files dan install dependensi
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy semua source code
COPY . .

# Build binary server + alat migrasi (migrasi SQL di-embed ke binary).
# Deploy: jalankan "./migrate up" dulu (job/release step), baru "./main".
RUN go build -o main . && go build -o migrate ./cmd/migrate

# Jalankan binary
CMD ["./main"]
