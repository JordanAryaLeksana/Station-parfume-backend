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

# Build binary
RUN go build -o main .

# Jalankan binary
CMD ["./main"]
