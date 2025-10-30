// internal/order/order_model.go
package order

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Status Pesanan (Enum)
type OrderStatus string

const (
	StatusPending   OrderStatus = "PENDING"
	StatusProcessed OrderStatus = "PROCESSED"
	StatusFailed    OrderStatus = "FAILED"
)

type Order struct {
	ID         uuid.UUID   `gorm:"type:uuid;primary_key;"`
	ProductID  uuid.UUID   `gorm:"type:uuid;not null"`
	TotalPrice float64     `gorm:"type:decimal(10,2);not null"`
	Status     OrderStatus `gorm:"type:varchar(50);not null"`
	CreatedAt  time.Time   `gorm:"default:CURRENT_TIMESTAMP"`
}

// Hook GORM untuk membuat UUID baru sebelum create
func (order *Order) BeforeCreate(tx *gorm.DB) (err error) {
	order.ID = uuid.New()
	return
}
