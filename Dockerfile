# Stage 1: Build
FROM golang:1.20-alpine AS builder

WORKDIR /app

# Salin file modul
COPY go.mod ./

# Jalankan tidy. Ini akan membuat go.sum DI DALAM container
# dan mengunduh dependensi jika ada.
RUN go mod tidy

# Salin sisa source code
COPY . .

# Build binary aplikasi
# -o app = output file bernama 'app'
# CGO_ENABLED=0 = build murni Go tanpa C libraries
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./...

# Stage 2: Production
FROM alpine:latest

WORKDIR /app

# Salin binary 'app' dari stage builder
COPY --from=builder /app/app .

# Expose port (meskipun sudah di-map di docker-compose, ini adalah praktik baik)
EXPOSE 8080

# Jalankan aplikasi
CMD ["./app"]