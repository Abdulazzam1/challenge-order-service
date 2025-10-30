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

// Order adalah model domain dan GORM untuk tabel 'orders'
type Order struct {
	// PENAMBAHAN JSON TAG UNTUK FIX TEST FAILURE
	ID         uuid.UUID   `gorm:"type:uuid;primary_key;" json:"id"`
	ProductID  uuid.UUID   `gorm:"type:uuid;not null" json:"product_id"`
	TotalPrice float64     `gorm:"type:decimal(10,2);not null" json:"total_price"`
	Status     OrderStatus `gorm:"type:varchar(50);not null" json:"status"`
	CreatedAt  time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

// Hook GORM untuk membuat UUID baru sebelum create
func (order *Order) BeforeCreate(tx *gorm.DB) (err error) {
	// Jika ID masih kosong (uuid.Nil), generate ID baru.
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	// Tambahkan status default jika belum diset
	if order.Status == "" {
		order.Status = StatusPending
	}
	return
}
