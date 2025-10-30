package service

import (
	"challenge-order-service/internal/order"
	"challenge-order-service/internal/order/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync" // <-- Impor baru untuk cache yang aman
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

// Konteks global untuk Redis
var ctx = context.Background()

// --- INTERFACES UNTUK MOCKING ---
type Publisher interface {
	Publish(exchange, routingKey string, body []byte) error
}

type ProductServiceClient interface {
	GetProductInfo(productID uuid.UUID) (*ProductResponse, error)
}

// --- CORE SERVICE DEFINITIONS ---
type OrderService interface {
	CreateOrder(req order.CreateOrderRequest) (*order.Order, error)
	GetOrdersByProductID(productID uuid.UUID) ([]order.Order, error)
}

type ProductResponse struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price float64   `json:"price,string"`
	Qty   int       `json:"qty"`
}

// 2. Definisikan "Implementasi" (Struct)
type orderService struct {
	repo          repository.OrderRepository
	rdb           *redis.Client
	publisher     Publisher
	productClient ProductServiceClient
}

// 3. Buat "Constructor"
func NewOrderService(
	repo repository.OrderRepository,
	rdb *redis.Client,
	publisher Publisher,
	productClient ProductServiceClient,
) OrderService {
	return &orderService{
		repo:          repo,
		rdb:           rdb,
		publisher:     publisher,
		productClient: productClient,
	}
}

// 4. Implementasi "CreateOrder"
func (s *orderService) CreateOrder(req order.CreateOrderRequest) (*order.Order, error) {

	// Panggil service client (yang sekarang punya cache-nya sendiri)
	product, err := s.productClient.GetProductInfo(req.ProductID)
	if err != nil {
		return nil, err
	}

	if product.Qty < req.Quantity {
		return nil, fmt.Errorf("stok produk %s tidak mencukupi", req.ProductID.String())
	}

	totalPrice := product.Price * float64(req.Quantity)

	newOrder := &order.Order{
		ID:         uuid.New(),
		ProductID:  req.ProductID,
		TotalPrice: totalPrice,
		Status:     order.StatusPending,
	}

	// Simpan ke DB
	savedOrder, err := s.repo.Save(newOrder)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan order: %w", err)
	}

	// Publish event
	err = s.publisher.Publish("orders_exchange", "order.created", s.createEventBody(savedOrder, req.Quantity))
	if err != nil {
		log.Printf("PERINGATAN: Order %s berhasil disimpan, tapi GAGAL publish event: %v", savedOrder.ID, err)
	}

	// Hapus cache 'GetOrdersByProductID' (ini masih di Redis, tidak apa-apa)
	cacheKey := fmt.Sprintf("orders_by_product:%s", req.ProductID.String())
	s.rdb.Del(ctx, cacheKey)

	return savedOrder, nil
}

// 5. Implementasi "GetOrdersByProductID"
func (s *orderService) GetOrdersByProductID(productID uuid.UUID) ([]order.Order, error) {
	cacheKey := fmt.Sprintf("orders_by_product:%s", productID.String())

	val, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		log.Println("CACHE HIT untuk GetOrdersByProductID:", productID)
		var orders []order.Order
		if json.Unmarshal([]byte(val), &orders) == nil {
			return orders, nil
		}
	}
	log.Println("CACHE MISS untuk GetOrdersByProductID:", productID)

	orders, err := s.repo.FindByProductID(productID)
	if err != nil {
		return nil, err
	}

	jsonData, _ := json.Marshal(orders)
	s.rdb.Set(ctx, cacheKey, jsonData, 10*time.Minute)
	return orders, nil
}

// --- FUNGSI HELPER & IMPLEMENTASI CONCRETE UNTUK main.go ---

// createEventBody membuat payload event RabbitMQ
func (s *orderService) createEventBody(order *order.Order, quantity int) []byte {
	// ... (kode ini sama seperti sebelumnya, tidak perlu diubah)
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
	body, _ := json.Marshal(event)
	return body
}

// ===================================================================
// === PERBAIKAN SOLUSI LAIN: BUAT CACHE IN-MEMORY DI SINI ===
// ===================================================================

// ProductClientImpl sekarang memiliki cache map-nya sendiri
type ProductClientImpl struct {
	productCache map[uuid.UUID]*ProductResponse // Cache in-memory kita
	mu           sync.RWMutex                   // Mutex untuk melindungi map
}

// NewProductClientImpl TIDAK LAGI MEMBUTUHKAN REDIS
func NewProductClientImpl() *ProductClientImpl {
	return &ProductClientImpl{
		// Inisialisasi map
		productCache: make(map[uuid.UUID]*ProductResponse),
	}
}

// GetProductInfo sekarang menggunakan Go map, BUKAN Redis
func (c *ProductClientImpl) GetProductInfo(productID uuid.UUID) (*ProductResponse, error) {

	// 1. Coba Cache Read (dari Go map)
	c.mu.RLock() // Kunci untuk membaca
	product, found := c.productCache[productID]
	c.mu.RUnlock() // Buka kunci

	if found {
		log.Println("IN-MEMORY CACHE HIT (Product Info):", productID)
		return product, nil
	}

	// 2. HTTP Fallback (Cache Miss)
	log.Println("IN-MEMORY CACHE MISS (Product Info):", productID)
	productServiceURL := fmt.Sprintf("http://product-service:3000/products/%s", productID.String())

	resp, err := http.Get(productServiceURL)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi product-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product-service mengembalikan error %d", resp.StatusCode)
	}

	var newProduct ProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&newProduct); err != nil {
		return nil, fmt.Errorf("gagal decode respons product-service: %w", err)
	}

	// 3. Tulis ke Cache (Go map)
	c.mu.Lock() // Kunci untuk menulis
	c.productCache[productID] = &newProduct
	c.mu.Unlock() // Buka kunci

	return &newProduct, nil
}

// ===================================================================

// PublisherImpl mengimplementasikan Publisher (CONCRETE)
// (Kode ini sama seperti sebelumnya, tidak perlu diubah)
type PublisherImpl struct {
	ch *amqp.Channel
}

func NewPublisherImpl(ch *amqp.Channel) *PublisherImpl {
	return &PublisherImpl{ch: ch}
}

func (p *PublisherImpl) Publish(exchange, routingKey string, body []byte) error {
	return p.ch.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}
