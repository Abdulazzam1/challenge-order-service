// internal/order/handler/order_handler.go
package handler

import (
	// Impor package internal kita
	"challenge-order-service/internal/order"
	"challenge-order-service/internal/order/service"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 1. Definisikan "Implementasi" (Struct)
// Handler ini menampung service yang akan ia panggil.
type OrderHandler struct {
	service service.OrderService
}

// 2. Buat "Constructor" (yang dipanggil oleh main.go)
func NewOrderHandler(s service.OrderService) *OrderHandler {
	return &OrderHandler{
		service: s,
	}
}

// 3. Implementasi Handler "CreateOrder" (untuk POST /orders)
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req order.CreateOrderRequest

	// Validasi input JSON (productId dan quantity)
	// Gin akan otomatis menangani error jika 'required' tidak ada.
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Panggil logika bisnis di Service
	savedOrder, err := h.service.CreateOrder(req)
	if err != nil {
		// Jika service mengembalikan error (misal: produk tidak ada)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Sukses: Kembalikan 201 Created dengan data order yang baru
	c.JSON(http.StatusCreated, savedOrder)
}

// 4. Implementasi Handler "GetOrdersByProductID" (untuk GET /orders/product/:productid)
func (h *OrderHandler) GetOrdersByProductID(c *gin.Context) {

	// Ambil 'productid' dari parameter URL
	paramID := c.Param("productid")
	productID, err := uuid.Parse(paramID)
	if err != nil {
		// Jika ID yang diberikan bukan UUID yang valid
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Product ID format"})
		return
	}

	// Panggil logika bisnis di Service (yang memiliki cache)
	orders, err := h.service.GetOrdersByProductID(productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Sukses: Kembalikan 200 OK dengan daftar order
	c.JSON(http.StatusOK, orders)
}
