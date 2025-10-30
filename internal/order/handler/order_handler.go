package handler

import (
	"challenge-order-service/internal/order"
	"net/http"

	// PERBAIKAN: Import package service karena interface OrderService didefinisikan di sana.
	"challenge-order-service/internal/order/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// OrderHandler menampung interface bisnis (Service)
// Ini adalah implementasi Handler yang mengikuti Clean Architecture.
type OrderHandler struct {
	// Menggunakan service.OrderService (SOLUSI UNTUK COMPILATION ERROR)
	Service service.OrderService
}

// NewOrderHandler adalah constructor untuk handler.
// Menggunakan service.OrderService (SOLUSI UNTUK COMPILATION ERROR)
func NewOrderHandler(svc service.OrderService) *OrderHandler {
	return &OrderHandler{
		Service: svc,
	}
}

// CreateOrder menangani endpoint POST /orders
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req order.CreateOrderRequest

	// 1. Binding dan Validasi Input
	if err := c.ShouldBindJSON(&req); err != nil {
		// Mengembalikan 400 Bad Request
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format or missing field.", "details": err.Error()})
		return
	}

	// 2. Panggil Service Layer
	createdOrder, err := h.Service.CreateOrder(req)

	if err != nil {
		// 3. Penanganan Error dari Service
		// Mengembalikan 500 Internal Server Error untuk error dari layer di bawahnya
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. Sukses Response
	// PENTING: Mengembalikan objek 'createdOrder' (SOLUSI UNTUK TEST FAILURE)
	// Gin akan men-marshal struct ini menjadi JSON: {"id": "...", "product_id": "...", ...}
	c.JSON(http.StatusCreated, createdOrder)
}

// GetOrdersByProductID menangani endpoint GET /orders/product/:productID
func (h *OrderHandler) GetOrdersByProductID(c *gin.Context) {
	productIDParam := c.Param("productID")

	// 1. Validasi Parameter UUID
	productID, err := uuid.Parse(productIDParam)
	if err != nil {
		// Mengembalikan 400 Bad Request
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Product ID format."})
		return
	}

	// 2. Panggil Service Layer
	orders, err := h.Service.GetOrdersByProductID(productID)

	if err != nil {
		// Mengembalikan 500 Internal Server Error untuk error dari layer di bawahnya
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. Sukses Response
	c.JSON(http.StatusOK, orders)
}
