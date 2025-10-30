# ---- Build Stage ----
# 1. PERBAIKAN: Gunakan versi Go yang cocok dengan go.mod
FROM golang:1.24-alpine AS builder

# Set up environment
WORKDIR /app
RUN apk add --no-cache git

# Salin file mod dan download dependensi
# Ini memanfaatkan Docker cache layer
COPY go.mod ./
COPY go.sum ./
RUN go mod tidy
RUN go mod download

# Salin sisa source code
COPY . .

# Build binary
# 2. PERBAIKAN: Path build menunjuk ke cmd/server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/order-service-binary ./cmd/server

# ---- Production Stage ----
# Gunakan image Alpine murni yang ringan
FROM alpine:latest  

WORKDIR /app

# Salin HANYA binary yang sudah di-build dari 'builder' stage
COPY --from=builder /app/order-service-binary .

# (Opsional) Salin file .env jika ada (meskipun docker-compose lebih baik)
# COPY .env .

# Port yang akan diekspos oleh Gin
EXPOSE 8080

# Perintah untuk menjalankan binary
CMD ["./order-service-binary"]