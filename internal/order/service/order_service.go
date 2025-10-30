// internal/order/service/order_service.go
package service

import (
	"challenge-order-service/internal/order"
	"challenge-order-service/internal/order/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

// Konteks global untuk Redis
var ctx = context.Background()

// 1. Definisikan "Kontrak" (Interface) Service
// Ini memberi tahu Handler fungsi apa saja yang tersedia.
type OrderService interface {
	CreateOrder(req order.CreateOrderRequest) (*order.Order, error)
	GetOrdersByProductID(productID uuid.UUID) ([]order.Order, error)
}

// 2. Definisikan "Implementasi" (Struct)
// Struct ini menampung semua "alat" yang dibutuhkan service:
// repo (DB), rdb (Redis), dan ch (RabbitMQ)
type orderService struct {
	repo repository.OrderRepository
	rdb  *redis.Client
	ch   *amqp.Channel
}

// 3. Buat "Constructor" (yang dipanggil oleh main.go)
func NewOrderService(repo repository.OrderRepository, rdb *redis.Client, ch *amqp.Channel) OrderService {
	return &orderService{
		repo: repo,
		rdb:  rdb,
		ch:   ch,
	}
}

// Struct internal untuk menampung data produk dari product-service
type ProductResponse struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price float64   `json:"price,string"` // Terima harga sebagai string agar fleksibel
	Qty   int       `json:"qty"`
}

// --- Implementasi Logika Inti ---

// 4. Implementasi "CreateOrder" (untuk POST /orders)
func (s *orderService) CreateOrder(req order.CreateOrderRequest) (*order.Order, error) {

	// TUGAS 1: Fetch info produk (untuk harga & validasi)
	// Ini adalah strategi "Hybrid Cache Read" untuk 1000 req/detik
	product, err := s.fetchProductInfo(req.ProductID)
	if err != nil {
		return nil, err // Gagal jika produk tidak ada
	}

	// TUGAS 2: Hitung total harga
	totalPrice := product.Price * float64(req.Quantity)

	// TUGAS 3: Buat struct Order baru
	newOrder := &order.Order{
		ProductID:  req.ProductID,
		TotalPrice: totalPrice,
		Status:     order.StatusPending, // Status awal selalu PENDING
	}

	// TUGAS 4: Simpan order ke Database (Postgres)
	savedOrder, err := s.repo.Save(newOrder)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan order: %w", err)
	}

	// TUGAS 5: Publish event "order.created"
	err = s.publishOrderCreatedEvent(savedOrder, req.Quantity)
	if err != nil {
		// Jika publish gagal, kita catat errornya tapi JANGAN gagalkan transaksi.
		// Order sudah tersimpan, itu yang utama.
		log.Printf("PERINGATAN: Order %s berhasil disimpan, tapi GAGAL publish event: %v", savedOrder.ID, err)
	}

	// TUGAS 6: Hapus cache (jika ada) untuk GET /orders/product/:productid
	// Agar GET selanjutnya mengambil data baru.
	cacheKey := fmt.Sprintf("orders_by_product:%s", req.ProductID.String())
	s.rdb.Del(ctx, cacheKey)

	// Kembalikan order yang berhasil disimpan
	return savedOrder, nil
}

// 5. Implementasi "GetOrdersByProductID" (untuk GET /orders/product/:productid)
func (s *orderService) GetOrdersByProductID(productID uuid.UUID) ([]order.Order, error) {

	// TUGAS 1: Cek Cache (Redis)
	cacheKey := fmt.Sprintf("orders_by_product:%s", productID.String())
	val, err := s.rdb.Get(ctx, cacheKey).Result()

	if err == nil {
		// Cache HIT (Ditemukan)
		log.Println("CACHE HIT untuk GetOrdersByProductID:", productID)
		var orders []order.Order
		if json.Unmarshal([]byte(val), &orders) == nil {
			return orders, nil
		}
	}

	// Cache MISS (Tidak Ditemukan)
	log.Println("CACHE MISS untuk GetOrdersByProductID:", productID)

	// TUGAS 2: Ambil dari Database (Postgres)
	orders, err := s.repo.FindByProductID(productID)
	if err != nil {
		return nil, err
	}

	// TUGAS 3: Simpan ke Cache (Redis) untuk 10 menit
	jsonData, _ := json.Marshal(orders)
	s.rdb.Set(ctx, cacheKey, jsonData, 10*time.Minute)

	return orders, nil
}

// --- Fungsi Helper (Internal) ---

// fetchProductInfo mengambil data produk (Harga, Nama, dll)
func (s *orderService) fetchProductInfo(productID uuid.UUID) (*ProductResponse, error) {

	// STRATEGI KINERJA (untuk 1000 req/detik):
	// Kita Coba ambil info produk dari cache Redis yang dibuat oleh NESTJS.
	// Kunci cache ini HARUS SAMA dengan yang dibuat NestJS (yaitu URL).
	productCacheKey := fmt.Sprintf("/products/%s", productID.String())

	val, err := s.rdb.Get(ctx, productCacheKey).Result()
	if err == nil {
		// Cache HIT (Ditemukan)
		log.Println("CACHE HIT (Product Info):", productID)
		var product ProductResponse
		if json.Unmarshal([]byte(val), &product) == nil {
			return &product, nil
		}
	}

	// Cache MISS (Tidak Ditemukan)
	log.Println("CACHE MISS (Product Info):", productID)

	// STRATEGI FALLBACK: Lakukan panggilan HTTP ke product-service
	// URL 'http://product-service:3000' menggunakan nama layanan Docker Compose.
	productServiceURL := fmt.Sprintf("http://product-service:3000/products/%s", productID.String())

	resp, err := http.Get(productServiceURL)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi product-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product-service mengembalikan error %d", resp.StatusCode)
	}

	var product ProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("gagal decode respons product-service: %w", err)
	}

	// (NestJS akan otomatis menyimpan ini ke cache, jadi panggilan selanjutnya akan HIT)
	return &product, nil
}

// publishOrderCreatedEvent mengirim pesan ke RabbitMQ
func (s *orderService) publishOrderCreatedEvent(order *order.Order, quantity int) error {
	// Buat payload sesuai "kontrak"
	event := struct {
		OrderID         string `json:"orderId"`
		ProductID       string `json:"productId"`
		QuantityOrdered int    `json:"quantityOrdered"`
		Timestamp       string `json:"timestamp"`
	}{
		OrderID:         order.ID.String(),
		ProductID:       order.ProductID.String(),
		QuantityOrdered: quantity,
		Timestamp:       time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("gagal serialize event: %w", err)
	}

	// Publish ke exchange
	return s.ch.Publish(
		"orders_exchange", // exchange
		"order.created",   // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}
