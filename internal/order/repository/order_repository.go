// internal/order/repository/order_repository.go
package repository

import (
	// Impor struct Order dari folder model kita
	"challenge-order-service/internal/order"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 1. Definisikan "Kontrak" (Interface)
type OrderRepository interface {
	Save(order *order.Order) (*order.Order, error)
	FindByProductID(productID uuid.UUID) ([]order.Order, error)
}

// 2. Definisikan "Implementasi" (Struct)
type orderRepository struct {
	db *gorm.DB
}

// 3. Buat "Constructor" (yang dipanggil oleh main.go)
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

// 4. Implementasikan fungsi "Save" (untuk POST /orders)
func (r *orderRepository) Save(order *order.Order) (*order.Order, error) {
	if err := r.db.Create(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}

// 5. Implementasikan fungsi "FindByProductID" (untuk GET /orders/product/:productid)
func (r *orderRepository) FindByProductID(productID uuid.UUID) ([]order.Order, error) {
	var orders []order.Order

	if err := r.db.Where("product_id = ?", productID).Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}
