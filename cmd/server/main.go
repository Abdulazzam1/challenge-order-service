// cmd/server/main.go
package main

import (
	"challenge-order-service/internal/order"
	"challenge-order-service/internal/order/handler"
	"challenge-order-service/internal/order/repository"
	"challenge-order-service/internal/order/service"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp" // <-- Pastikan ini 'streadway'
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Konteks global untuk klien Redis
var ctx = context.Background()

func main() {
	log.Println("Starting Order Service (Fase 4)...")

	// === 1. KONEKSI DATABASE ===
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatalf("DATABASE_URL environment variable is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connection established.")

	log.Println("Running AutoMigration...")
	db.AutoMigrate(&order.Order{})

	// 2. Inisialisasi Cache (Redis)
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Tes koneksi Redis
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Redis connection established.")

	// 3. Inisialisasi Message Broker (RabbitMQ)
	amqpURL := os.Getenv("RABBITMQ_URL")
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ channel: %v", err)
	}
	defer ch.Close()
	log.Println("RabbitMQ connection established.")

	// Deklarasikan exchange
	err = ch.ExchangeDeclare(
		"orders_exchange", // name
		"topic",           // type
		true,              // durable
		false,             // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare 'orders_exchange': %v", err)
	}

	// 4. Setup Listener 'order.created'
	go startOrderCreatedLogger(ch)

	// 5. Setup Arsitektur (Repository -> Service -> Handler)
	orderRepo := repository.NewOrderRepository(db)

	// FIX: Buat concrete implementation untuk 2 interface baru
	productClient := service.NewProductClientImpl(rdb)
	publisher := service.NewPublisherImpl(ch)

	// FIX: Panggil dengan 4 argumen baru
	// NewOrderService(repo, rdb, publisher, productClient)
	orderService := service.NewOrderService(orderRepo, rdb, publisher, productClient)

	orderHandler := handler.NewOrderHandler(orderService)

	// 6. Setup Gin Router
	router := gin.Default()
	router.SetTrustedProxies(nil)

	// Rute Health Check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Rute Fase 4
	api := router.Group("/api/v1")
	{
		api.POST("/orders", orderHandler.CreateOrder)
		api.GET("/orders/product/:productid", orderHandler.GetOrdersByProductID)
	}

	// Menjalankan server
	log.Println("Order Service (Fase 4) is running on :8080")
	router.Run(":8080")
}

// startOrderCreatedLogger adalah fitur dari soal PDF:
// "order-service should listen for order.created events and log them"
func startOrderCreatedLogger(ch *amqp.Channel) {
	q, err := ch.QueueDeclare(
		"q.orders.log", // name (buat queue baru untuk logging)
		true,           // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		log.Printf("Failed to declare queue 'q.orders.log': %v", err)
		return
	}

	// Bind queue ini ke exchange yang sama dengan routing key yang sama
	err = ch.QueueBind(
		q.Name,            // queue name
		"order.created",   // routing key
		"orders_exchange", // exchange
		false,
		nil,
	)
	if err != nil {
		log.Printf("Failed to bind queue 'q.orders.log': %v", err)
		return
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Printf("Failed to register consumer for 'q.orders.log': %v", err)
		return
	}

	log.Println("Goroutine (Logger) for 'order.created' started...")
	// Loop selamanya untuk mendengarkan pesan
	for d := range msgs {
		log.Printf("[EVENT LOGGER] Received 'order.created' event: %s", d.Body)
	}
}
