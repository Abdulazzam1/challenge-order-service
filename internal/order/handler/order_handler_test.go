package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"challenge-order-service/internal/order"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- MOCK SERVICE (Kontrak untuk Handler) ---

// MockOrderService adalah mock untuk service.OrderService
type MockOrderService struct {
	mock.Mock
}

// CreateOrder: Mock sesuai interface service
func (m *MockOrderService) CreateOrder(req order.CreateOrderRequest) (*order.Order, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*order.Order), args.Error(1)
}

// GetOrdersByProductID: Mock sesuai interface service
func (m *MockOrderService) GetOrdersByProductID(productID uuid.UUID) ([]order.Order, error) {
	args := m.Called(productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]order.Order), args.Error(1)
}

// --- SETUP TEST ---

// setupTest membuat Handler baru dan Gin engine (tanpa menjalankan server)
func setupTest(mockSvc *MockOrderService) (*gin.Engine, *OrderHandler) {
	// Ganti mode ke test untuk menonaktifkan log Gin
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Gunakan NewOrderHandler (pastikan signature di handler.go sesuai)
	handler := NewOrderHandler(mockSvc)

	// Definisikan endpoint sesuai main.go
	router.POST("/orders", handler.CreateOrder)
	router.GET("/orders/product/:productID", handler.GetOrdersByProductID)

	return router, handler
}

// --- TEST CASES: POST /orders ---

func TestCreateOrder_Success(t *testing.T) {
	mockSvc := new(MockOrderService)
	router, _ := setupTest(mockSvc)

	testProductID := uuid.New()
	reqBody := order.CreateOrderRequest{
		ProductID: testProductID,
		Quantity:  5,
	}

	expectedOrder := &order.Order{
		ID:         uuid.New(),
		ProductID:  testProductID,
		TotalPrice: 500.00,
	}

	// 1. Arrange: Mock Service (memastikan Handler memanggil Service dengan benar)
	mockSvc.On("CreateOrder", reqBody).Return(expectedOrder, nil).Once()

	// 2. Act
	reqBodyJSON, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(reqBodyJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 3. Assert
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseBody map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &responseBody)
	assert.NoError(t, err)
	assert.Equal(t, expectedOrder.ID.String(), responseBody["id"])

	mockSvc.AssertExpectations(t)
}

func TestCreateOrder_InvalidInput(t *testing.T) {
	mockSvc := new(MockOrderService)
	router, _ := setupTest(mockSvc)

	// 1. Arrange: Request Body invalid (misal, Quantity 0)
	reqBody := map[string]interface{}{
		"product_id": uuid.New().String(),
		"quantity":   0, // Invalid Quantity
	}

	// 2. Act
	reqBodyJSON, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(reqBodyJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 3. Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Pastikan Service TIDAK dipanggil karena gagal di tahap validasi Handler
	mockSvc.AssertNotCalled(t, "CreateOrder", mock.Anything)
}

func TestCreateOrder_ServiceError(t *testing.T) {
	mockSvc := new(MockOrderService)
	router, _ := setupTest(mockSvc)

	testProductID := uuid.New()
	reqBody := order.CreateOrderRequest{
		ProductID: testProductID,
		Quantity:  5,
	}

	// 1. Arrange: Mock Service GAGAL (simulasi produk tidak ditemukan, dll)
	svcErr := errors.New("produk tidak ditemukan")
	mockSvc.On("CreateOrder", reqBody).Return(nil, svcErr).Once()

	// 2. Act
	reqBodyJSON, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(reqBodyJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 3. Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code) // Atau 404/400 tergantung penanganan error

	mockSvc.AssertExpectations(t)
}
