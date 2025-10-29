package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
    "context" // Tambahkan ini
    "time"    // Tambahkan ini

	amqp "github.com/rabbitmq/amqp091-go" // Import library baru
)

func main() {
	// Daftarkan handler untuk route '/health'
	http.HandleFunc("/health", healthCheckHandler)

	// Daftarkan handler untuk route '/test-event' BARU
	http.HandleFunc("/test-event", testEventHandler)

	log.Println("Go (order-service) is running on :8080")
	// Mulai server di port 8080
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}

// handler untuk /health
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"status": "ok"}
	json.NewEncoder(w).Encode(response)
}

// HANDLER BARU: /test-event
func testEventHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Dapatkan URL RabbitMQ dari environment variable
	//    (Ini diset di docker-compose.yml)
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		// Fallback jika env var tidak ada
		rabbitMQURL = "amqp://guest:guest@localhost:5672/"
	}

	// 2. Koneksi ke RabbitMQ
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		http.Error(w, "Failed to connect to RabbitMQ", http.StatusInternalServerError)
		log.Printf("Failed to connect to RabbitMQ: %s", err)
		return
	}
	defer conn.Close()

	// 3. Buat Channel
	ch, err := conn.Channel()
	if err != nil {
		http.Error(w, "Failed to open a channel", http.StatusInternalServerError)
		log.Printf("Failed to open a channel: %s", err)
		return
	}
	defer ch.Close()

	// 4. Deklarasikan Exchange (sesuai "kontrak" Fase 0)
	//    Ini memastikan exchange-nya ada.
	err = ch.ExchangeDeclare(
		"orders_exchange", // name (nama exchange)
		"topic",           // type
		true,              // durable (bertahan jika broker restart)
		false,             // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		http.Error(w, "Failed to declare an exchange", http.StatusInternalServerError)
		log.Printf("Failed to declare an exchange: %s", err)
		return
	}

	// 5. Siapkan pesan "halo" kita
	body := `{"message": "hello from Go!"}`
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

	// 6. Publish pesan
	err = ch.PublishWithContext(ctx,
		"orders_exchange", // exchange
		"order.created",   // routing key (sesuai "kontrak" Fase 0)
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(body),
		},
	)
	if err != nil {
		http.Error(w, "Failed to publish a message", http.StatusInternalServerError)
		log.Printf("Failed to publish a message: %s", err)
		return
	}

	// 7. Beri respons sukses
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"status": "test event published!"}
	json.NewEncoder(w).Encode(response)
	log.Println("Test event published successfully!")
}