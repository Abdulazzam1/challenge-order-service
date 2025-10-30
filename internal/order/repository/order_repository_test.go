package repository_test

import (
	"testing"
	"time"

	// Impor struct Order dari direktori internal/order
	"challenge-order-service/internal/order"
	// Impor repository yang akan diuji
	"challenge-order-service/internal/order/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB menginisialisasi database SQLite in-memory untuk pengujian
func setupTestDB(t *testing.T) *gorm.DB {
	// 1. Buka koneksi ke SQLite in-memory
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err, "Gagal membuka koneksi DB in-memory")

	// 2. Melakukan AutoMigrate untuk membuat tabel Order
	err = db.AutoMigrate(&order.Order{})
	assert.NoError(t, err, "Gagal melakukan AutoMigrate untuk tabel Order")

	return db
}

// ====================================================================
// TEST CASE: Save
// ====================================================================
func TestOrderRepository_Save_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)

	testProductID := uuid.New()

	// 1. Arrange: Buat objek Order baru
	newOrder := &order.Order{
		ProductID:  testProductID,
		TotalPrice: 99.99,
		Status:     order.StatusPending,
		CreatedAt:  time.Now(),
	}

	// 2. Act: Panggil metode Save
	savedOrder, err := repo.Save(newOrder)

	// 3. Assert
	assert.NoError(t, err, "Save seharusnya tidak mengembalikan error")
	assert.NotNil(t, savedOrder, "Saved Order seharusnya tidak nil")
	assert.NotEqual(t, uuid.Nil, savedOrder.ID, "Order ID seharusnya sudah di-generate oleh BeforeCreate hook")

	// Verifikasi data di-database dengan query langsung
	var fetchedOrder order.Order
	err = db.First(&fetchedOrder, "id = ?", savedOrder.ID).Error
	assert.NoError(t, err, "Order yang disimpan seharusnya dapat ditemukan di DB")
	assert.Equal(t, savedOrder.ID, fetchedOrder.ID)
	assert.Equal(t, newOrder.TotalPrice, fetchedOrder.TotalPrice)
}

// ====================================================================
// TEST CASE: FindByProductID
// ====================================================================
func TestOrderRepository_FindByProductID_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)

	testProductID := uuid.New()
	anotherProductID := uuid.New()

	// 1. Arrange: Siapkan dan simpan data fixture
	ordersToSave := []order.Order{
		{ProductID: testProductID, TotalPrice: 10.00, Status: order.StatusPending, CreatedAt: time.Now()},    // #1: Akan ditemukan
		{ProductID: testProductID, TotalPrice: 20.00, Status: order.StatusProcessed, CreatedAt: time.Now()},  // #2: Akan ditemukan
		{ProductID: anotherProductID, TotalPrice: 50.00, Status: order.StatusPending, CreatedAt: time.Now()}, // #3: Tidak akan ditemukan
	}

	for _, o := range ordersToSave {
		_, err := repo.Save(&o)
		assert.NoError(t, err, "Gagal menyimpan order fixture")
	}

	// 2. Act: Cari order berdasarkan ProductID
	foundOrders, err := repo.FindByProductID(testProductID)

	// 3. Assert
	assert.NoError(t, err, "FindByProductID seharusnya tidak mengembalikan error")
	assert.Len(t, foundOrders, 2, "Seharusnya menemukan 2 order untuk ProductID yang diuji")

	for _, o := range foundOrders {
		assert.Equal(t, testProductID, o.ProductID, "Semua order yang dikembalikan harus memiliki ProductID yang sama")
	}
}

func TestOrderRepository_FindByProductID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)

	// 1. Arrange: DB kosong atau tanpa data untuk ProductID ini
	nonExistentProductID := uuid.New()

	// 2. Act
	foundOrders, err := repo.FindByProductID(nonExistentProductID)

	// 3. Assert
	assert.NoError(t, err, "Tidak menemukan record seharusnya tidak dianggap error oleh Repository Find")
	assert.Empty(t, foundOrders, "Seharusnya mengembalikan slice kosong jika tidak ditemukan")
}
