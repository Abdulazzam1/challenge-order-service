// internal/order/order_model_test.go
package order

import (
	"testing" // Hanya perlu 'testing'

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Ini adalah satu-satunya unit test yang kita perlukan untuk Go.
func TestOrder_BeforeCreate(t *testing.T) {
	// Buat order baru
	order := &Order{
		// ID awalnya adalah 'nil'
		ID: uuid.Nil,
	}

	// Panggil fungsi yang ingin kita tes
	// Kita bisa loloskan 'nil' untuk 'tx' karena fungsinya tidak menggunakannya
	err := order.BeforeCreate(nil)

	// Verifikasi hasil
	assert.NoError(t, err)                 // Pastikan tidak ada error
	assert.NotEqual(t, uuid.Nil, order.ID) // Pastikan ID telah di-generate
}
