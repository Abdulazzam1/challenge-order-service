Anda benar. Output `go test ./...` yang baru itu jauh lebih baik karena menunjukkan bahwa *semua* paket Anda berhasil di-compile (`ok`), bukan hanya folder `order`.

Berikut adalah `README.md` versi final yang telah diperbarui dengan log `go test ./...` yang baru Anda kirim.

````markdown
# Tantangan Uji Full Stack: Arsitektur Microservice NestJS & Go

Ini adalah implementasi untuk tantangan uji full stack, yang membangun aplikasi berbasis 5 tumpukan (stack) teknologi yang diorkestrasi oleh Docker Compose.

* **Layanan Produk (NestJS):** `challenge-product-service`
* **Layanan Pesanan (Go):** `challenge-order-service`

## 1. Arsitektur

Aplikasi ini menggunakan arsitektur microservice berbasis *event-driven*:

1.  **`product-service` (NestJS):** Bertanggung jawab atas CRUD produk.
2.  **`order-service` (Go):** Bertanggung jawab atas CRUD pesanan.
3.  **`PostgreSQL`:** Database utama untuk kedua layanan.
4.  **`RabbitMQ`:** Berfungsi sebagai *message broker*.
5.  **`Redis`:** Berfungsi sebagai *caching layer*.

### Alur Kerja (Event-Driven)

Alur utama (`POST /orders`) dirancang untuk asinkron:
1.  Klien mengirim `POST /api/v1/orders` ke **`order-service` (Go)**.
2.  Layanan Go menyimpan pesanan ke DB (status `PENDING`) dan segera mem-publish event `order.created` ke **RabbitMQ**.
3.  **`product-service` (NestJS)** mendengarkan event `order.created` tersebut.
4.  Setelah menerima event, NestJS mengurangi `qty` produk di databasenya dan menghapus *cache* produk yang relevan.

## 2. Cara Menjalankan

### Prasyarat
* Docker & Docker Compose
* Git

### Langkah-langkah
1.  Clone kedua repositori ke dalam folder yang sama (berdampingan):
    ```bash
    git clone [URL_REPO_PRODUCT_SERVICE]
    git clone [URL_REPO_ORDER_SERVICE]
    ```

2.  Masuk ke folder `challenge-order-service`:
    ```bash
    cd challenge-order-service
    ```

3.  Jalankan Docker Compose:
    ```bash
    docker-compose up --build -d
    ```
    Perintah ini akan membangun *image* untuk kedua layanan dan menjalankan semua 5 kontainer.

## 3. Contoh API (cURL)

### a. Membuat Produk Baru
```bash
curl --location 'http://localhost:3000/products' \
--header 'Content-Type: application/json' \
--data '{
    "name": "Laptop Demo",
    "price": 1500,
    "qty": 50
}'
````

### b. Mengambil Produk (Cached)

(Gunakan ID yang didapat dari langkah 'a')

```bash
curl --location 'http://localhost:3000/products/[ID_PRODUK_ANDA]'
```

### c. Membuat Pesanan Baru

(Gunakan ID yang didapat dari langkah 'a')

```bash
curl --location 'http://localhost:8080/api/v1/orders' \
--header 'Content-Type: application/json' \
--data '{
    "productId": "[ID_PRODUK_ANDA]",
    "quantity": 2
}'
```

### d. Mengambil Pesanan per Produk (Cached)

(Gunakan ID yang didapat dari langkah 'a')

```bash
curl --location 'http://localhost:8080/api/v1/orders/product/[ID_PRODUK_ANDA]'
```

## 4\. Hasil Pengujian

### 4.1. Tes Fungsional (End-to-End)

Semua alur fungsional telah divalidasi secara manual:

  * `POST /orders` (Go) berhasil memicu pengurangan `qty` di `product-service` (NestJS) melalui RabbitMQ, yang dibuktikan dengan berkurangnya stok produk saat diperiksa.

### 4.2. Tes Unit

Kedua layanan telah memenuhi syarat "setidaknya satu unit test":

**`product-service` (NestJS):**

```bash
> npm test

 PASS  src/product/product.service.spec.ts
 Test Suites: 1 passed, 1 total
 Tests:       1 passed, 1 total
```

**`order-service` (Go):**
(Log ini menunjukkan bahwa tes unit di `internal/order` lulus (`ok`) dan semua paket kode lainnya berhasil di-compile.)

```bash
> go test ./...

?      challenge-order-service/cmd/server       [no test files]
ok     challenge-order-service/internal/order   (cached)
ok     challenge-order-service/internal/order/handler  0.255s
ok     challenge-order-service/internal/order/repository       (cached)
ok     challenge-order-service/internal/order/service  1.276s
```

### 4.3. Hasil Tes Kinerja & Kekurangan

  * **Target:** `1000 requests/seconds` untuk `POST /orders`.
  * **Hasil:** Gagal. Layanan mengalami *bottleneck* parah di **\~120-130 req/s** dan sebagian besar *request* mengalami `timeout`.
  * **Diagnosis:** *Bottleneck* ini disebabkan oleh tiga operasi I/O sinkron (Tulis DB, Tulis RabbitMQ, Hapus Redis) yang terjadi pada setiap *request*. Untuk mencapai 1000 req/s, arsitektur `order-service` perlu diubah agar penulisan ke database (`repo.Save`) terjadi secara asinkron di *background worker*.

<!-- end list -->

```
```