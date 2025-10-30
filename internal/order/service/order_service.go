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

// --- INTERFACES BARU UNTUK MOCKING (Wajib) ---

// Publisher mendefinisikan kontrak untuk pengiriman pesan (RabbitMQ/Kafka)
type Publisher interface {
	Publish(exchange, routingKey string, body []byte) error
}

// ProductServiceClient mendefinisikan kontrak untuk mendapatkan info produk (Hybrid Cache Read)
type ProductServiceClient interface {
	// Menghilangkan context di sini agar sesuai dengan signature yang lebih sederhana
	GetProductInfo(productID uuid.UUID) (*ProductResponse, error)
}

// --- CORE SERVICE DEFINITIONS ---

// 1. Definisikan "Kontrak" (Interface) Service
type OrderService interface {
	CreateOrder(req order.CreateOrderRequest) (*order.Order, error)
	GetOrdersByProductID(productID uuid.UUID) ([]order.Order, error)
}

// Struct internal untuk menampung data produk
type ProductResponse struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Price float64   `json:"price,string"` // Asumsi dari NestJS
	Qty   int       `json:"qty"`
}

// 2. Definisikan "Implementasi" (Struct)
type orderService struct {
	repo          repository.OrderRepository
	rdb           *redis.Client
	publisher     Publisher            // Menggunakan Interface
	productClient ProductServiceClient // Menggunakan Interface
}

// 3. Buat "Constructor" BARU (4 DEPENDENCY UNTUK MOCKING)
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

// --- Implementasi Logika Inti ---

// 4. Implementasi "CreateOrder"
func (s *orderService) CreateOrder(req order.CreateOrderRequest) (*order.Order, error) {

	// TUGAS 1: Fetch info produk MENGGUNAKAN INTERFACE BARU (tanpa context)
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

	// FIX: Hapus `context` dari panggilan Save (Menyelesaikan WrongArgCount Line 104)
	savedOrder, err := s.repo.Save(newOrder)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan order: %w", err)
	}

	// TUGAS 5: Publish event MENGGUNAKAN INTERFACE BARU
	err = s.publisher.Publish("orders_exchange", "order.created", s.createEventBody(savedOrder, req.Quantity))
	if err != nil {
		log.Printf("PERINGATAN: Order %s berhasil disimpan, tapi GAGAL publish event: %v", savedOrder.ID, err)
		// Tetap sukses karena event publishing bersifat async dan tidak memblokir order
	}

	// TUGAS 6: Hapus cache GetOrdersByProductID
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

	// FIX: Ganti GetOrdersByProductID menjadi FindByProductID (Menyelesaikan MissingFieldOrMethod Line 142)
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

// ProductClientImpl mengimplementasikan ProductServiceClient (CONCRETE)
type ProductClientImpl struct {
	rdb *redis.Client
}

func NewProductClientImpl(rdb *redis.Client) *ProductClientImpl {
	return &ProductClientImpl{rdb: rdb}
}

func (c *ProductClientImpl) GetProductInfo(productID uuid.UUID) (*ProductResponse, error) {
	// Logika Hybrid Cache Read (Cache + HTTP Fallback)
	productCacheKey := fmt.Sprintf("/products/%s", productID.String())

	// 1. Coba Cache Read
	val, err := c.rdb.Get(ctx, productCacheKey).Result()
	if err == nil {
		log.Println("CACHE HIT (Product Info):", productID)
		var product ProductResponse
		if json.Unmarshal([]byte(val), &product) == nil {
			return &product, nil
		}
	}

	// 2. HTTP Fallback
	log.Println("CACHE MISS (Product Info):", productID)
	// Ganti URL ini jika environment Anda berbeda
	productServiceURL := fmt.Sprintf("http://product-service:3000/products/%s", productID.String())

	resp, err := http.Get(productServiceURL)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi product-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("produk tidak ditemukan")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product-service mengembalikan error %d", resp.StatusCode)
	}

	var product ProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("gagal decode respons product-service: %w", err)
	}

	return &product, nil
}

// PublisherImpl mengimplementasikan Publisher (CONCRETE)
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
