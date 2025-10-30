package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Definisikan tipe "kontrak" kita (juga digunakan di NestJS)
type OrderCreatedEvent struct {
	OrderID         string `json:"orderId"`
	ProductID       string `json:"productId"`
	QuantityOrdered int    `json:"quantityOrdered"`
	Timestamp       string `json:"timestamp"`
}

func main() {
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/test-event", testEventHandler) // Endpoint ini sekarang lebih pintar

	log.Println("Go (order-service) is running on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"status": "ok"}
	json.NewEncoder(w).Encode(response)
}

// --- PERUBAHAN DI SINI ---
func testEventHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Baca payload JSON dari body request (Postman)
	var payload OrderCreatedEvent
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Error decoding body: %s", err)
		return
	}
	// Tambahkan data yang hilang (kita buat dummy saja)
	payload.OrderID = "dummy-order-id"
	payload.Timestamp = time.Now().Format(time.RFC3339)

	// 2. Koneksi ke RabbitMQ (sama seperti sebelumnya)
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		rabbitMQURL = "amqp://guest:guest@localhost:5672/"
	}
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		http.Error(w, "Failed to connect to RabbitMQ", http.StatusInternalServerError)
		log.Printf("Failed to connect to RabbitMQ: %s", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		http.Error(w, "Failed to open a channel", http.StatusInternalServerError)
		log.Printf("Failed to open a channel: %s", err)
		return
	}
	defer ch.Close()

	// 3. Deklarasikan Exchange (sama seperti sebelumnya)
	err = ch.ExchangeDeclare(
		"orders_exchange", "topic", true, false, false, false, nil,
	)
	if err != nil {
		http.Error(w, "Failed to declare an exchange", http.StatusInternalServerError)
		log.Printf("Failed to declare an exchange: %s", err)
		return
	}

	// 4. Ubah pesan 'body' menjadi payload dari Postman
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to serialize payload", http.StatusInternalServerError)
		log.Printf("Error serializing payload: %s", err)
		return
	}

	// 5. Publish pesan (sama seperti sebelumnya)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = ch.PublishWithContext(ctx,
		"orders_exchange", "order.created", false, false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body, // <-- Menggunakan body dari Postman
		},
	)
	if err != nil {
		http.Error(w, "Failed to publish a message", http.StatusInternalServerError)
		log.Printf("Failed to publish a message: %s", err)
		return
	}

	// 6. Beri respons sukses
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"status": "real test event published!"}
	json.NewEncoder(w).Encode(response)
	log.Printf("Real test event published successfully: %s", string(body))
}
