package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"challenge-order-service/internal/order"
	"challenge-order-service/internal/order/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- MOCK DEFINITIONS ---

// MockProductService adalah mock untuk interface ProductServiceClient
type MockProductService struct {
	mock.Mock
}

func (m *MockProductService) GetProductInfo(productID uuid.UUID) (*ProductResponse, error) {
	args := m.Called(productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ProductResponse), args.Error(1)
}

// MockPublisher adalah mock untuk interface Publisher
type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(exchange, routingKey string, body []byte) error {
	args := m.Called(exchange, routingKey, body)
	return args.Error(0)
}

// --- CONSTANTS & DATA HELPER ---

var testProductID = uuid.MustParse("a609d17d-7b24-4f40-b615-5e6f3d9a1f28")
var testOrderID = uuid.MustParse("b7c8e9f0-1234-5678-9abc-def012345678")

const (
	testPrice    = 100.00
	testQuantity = 5
)

// Helper untuk mendapatkan key cache order
func getOrdersCacheKey(id uuid.UUID) string {
	return fmt.Sprintf("orders_by_product:%s", id.String())
}

// Helper untuk mendapatkan key cache produk
func getProductCacheKey(id uuid.UUID) string {
	return fmt.Sprintf("/products/%s", id.String())
}

// --- TEST SETUP ---

func setupTest(t *testing.T) (OrderService, *repository.MockOrderRepository, *MockPublisher, *miniredis.Miniredis, *MockProductService) {
	// 1. Setup Mock Repository & Publisher & Product Client
	mockRepo := new(repository.MockOrderRepository)
	mockPublisher := new(MockPublisher)
	mockProductClient := new(MockProductService)

	// 2. Setup Mock Redis (miniredis)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// 3. Create Service - Menggunakan 4 ARGUMEN SESUAI CONSTRUCTOR BARU
	svc := NewOrderService(mockRepo, rdb, mockPublisher, mockProductClient)

	return svc, mockRepo, mockPublisher, mr, mockProductClient
}

// --- TEST CASES: CreateOrder ---

// Diubah namanya menjadi Success biasa, karena kita mem-mock klien produk secara langsung
func TestOrderService_CreateOrder_Success(t *testing.T) {
	// PENTING: Gunakan mockProductClient
	svc, mockRepo, mockPublisher, mr, mockProductClient := setupTest(t)
	defer mr.Close()

	// 1. Arrange: Siapkan data produk untuk di-mock
	productInfo := ProductResponse{ID: testProductID, Name: "Test Product", Price: testPrice, Qty: 50}

	// Hasil yang diharapkan dari Save (dengan ID yang sudah terisi)
	expectedOrder := &order.Order{
		ID:         testOrderID,
		ProductID:  testProductID,
		TotalPrice: testPrice * float64(testQuantity),
		Status:     order.StatusPending,
	}

	// 2. Arrange: Mock Product Client (FIX: Menambahkan ekspektasi yang hilang)
	mockProductClient.On("GetProductInfo", testProductID).
		Return(&productInfo, nil).Once()

	// 3. Arrange: Mock Repository (Save berhasil)
	mockRepo.On("Save", mock.AnythingOfType("*order.Order")).
		Return(expectedOrder, nil).Once()

	// 4. Mock Publisher (Publish berhasil)
	mockPublisher.On("Publish", "orders_exchange", "order.created", mock.AnythingOfType("[]uint8")).
		Return(nil).Once()

	// 5. Act
	createReq := order.CreateOrderRequest{ProductID: testProductID, Quantity: testQuantity}

	newOrder, err := svc.CreateOrder(createReq)

	// 6. Assert
	assert.NoError(t, err)
	assert.NotNil(t, newOrder)

	// Verifikasi mock yang dipanggil (Pastikan GetProductInfo dipanggil)
	mockProductClient.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestOrderService_CreateOrder_ProductInfoFails(t *testing.T) {
	// FIX: Menggunakan '_' untuk mockRepo dan mockPublisher
	svc, _, _, mr, mockProductClient := setupTest(t)
	defer mr.Close()

	// 1. Arrange: Mock Klien Produk GAGAL
	httpErr := errors.New("product service unavailable")
	mockProductClient.On("GetProductInfo", testProductID).
		Return(nil, httpErr).Once()

	// 2. Act
	createReq := order.CreateOrderRequest{ProductID: testProductID, Quantity: testQuantity}
	_, err := svc.CreateOrder(createReq)

	// 3. Assert
	assert.Error(t, err)

	mockProductClient.AssertExpectations(t)
}

// --- TEST CASES: GetOrdersByProductID ---

func TestOrderService_GetOrdersByProductID_CacheHit(t *testing.T) {
	// FIX: Menggunakan '_' untuk mockPublisher dan mockProductClient
	svc, mockRepo, _, mr, _ := setupTest(t)
	defer mr.Close()

	// 1. Arrange: Data Order
	expectedOrders := []order.Order{
		{ID: uuid.New(), ProductID: testProductID, TotalPrice: 1000},
	}
	ordersJSON, _ := json.Marshal(expectedOrders)

	// Pre-populate Redis (Ini adalah cache HIT yang valid, karena cache ini diakses LANGSUNG oleh OrderService)
	mr.Set(getOrdersCacheKey(testProductID), string(ordersJSON))

	// Pastikan Repository TIDAK dipanggil
	mockRepo.AssertNotCalled(t, "FindByProductID", mock.Anything)

	// 2. Act
	result, err := svc.GetOrdersByProductID(testProductID)

	// 3. Assert
	assert.NoError(t, err)
	assert.Equal(t, len(expectedOrders), len(result))
}

func TestOrderService_GetOrdersByProductID_CacheMiss(t *testing.T) {
	// FIX: Menggunakan '_' untuk mockPublisher dan mockProductClient
	svc, mockRepo, _, mr, _ := setupTest(t)
	defer mr.Close()

	// 1. Arrange: Redis kosong (CACHE MISS)

	// 2. Arrange: Mock Repository (akan dipanggil)
	expectedOrders := []order.Order{
		{ID: uuid.New(), ProductID: testProductID, TotalPrice: 1000},
	}
	mockRepo.On("FindByProductID", testProductID).
		Return(expectedOrders, nil).Once()

	// 3. Act
	result, err := svc.GetOrdersByProductID(testProductID)

	// 4. Assert
	assert.NoError(t, err)
	assert.Equal(t, len(expectedOrders), len(result))

	// Verifikasi data sekarang ada di Redis
	val, _ := mr.Get(getOrdersCacheKey(testProductID))
	assert.True(t, len(val) > 0, "Orders must be cached after cache miss")

	mockRepo.AssertExpectations(t)
}
