package repository

import (
	"challenge-order-service/internal/order"
	"context"

	"github.com/google/uuid"

	"github.com/stretchr/testify/mock"
)

// MockOrderRepository adalah mock untuk OrderRepositoryInterface
// Pastikan interface ini didefinisikan di order_repository.go
type MockOrderRepository struct {
	mock.Mock
}

// Save: Signature diperbaiki: TANPA CONTEXT dan mengembalikan (*order.Order, error).
// Ini menghilangkan error 'too many arguments in call to s.repo.Save' yang lama.
func (m *MockOrderRepository) Save(ord *order.Order) (*order.Order, error) {
	// Panggilan mock juga tanpa context
	args := m.Called(ord)

	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.(*order.Order), args.Error(1)
}

// FindByProductID: Ditambahkan/Diganti dari GetOrdersByProductID.
// Ini menghilangkan error 'missing method FindByProductID' di order_service_test.go.
func (m *MockOrderRepository) FindByProductID(productID uuid.UUID) ([]order.Order, error) {
	args := m.Called(productID)

	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]order.Order), args.Error(1)
}

// Catatan: Method GetOrdersByProductID yang lama dipertahankan di mock
// jika Anda masih menggunakannya di tempat lain atau untuk memudahkan transisi,
// tetapi panggilan ke DB/Repo di Service sudah menggunakan FindByProductID.
func (m *MockOrderRepository) GetOrdersByProductID(ctx context.Context, productID string) ([]order.Order, error) {
	args := m.Called(ctx, productID)
	result := args.Get(0)
	if result == nil {
		return nil, args.Error(1)
	}
	return result.([]order.Order), args.Error(1)
}
