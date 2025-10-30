// internal/order/order_dto.go
package order

import (
	"time"

	"github.com/google/uuid"
)

// Payload JSON untuk POST /orders
type CreateOrderRequest struct {
	ProductID uuid.UUID `json:"productId" binding:"required"`
	Quantity  int       `json:"quantity" binding:"required,min=1"`
}

// Response JSON untuk order yang berhasil dibuat
type OrderResponse struct {
	ID         uuid.UUID   `json:"id"`
	ProductID  uuid.UUID   `json:"productId"`
	TotalPrice float64     `json:"totalPrice"`
	Status     OrderStatus `json:"status"`
	CreatedAt  time.Time   `json:"createdAt"`
}
