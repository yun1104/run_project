package service

import (
	"context"
	"errors"
	"fmt"
	"meituan-ai-agent/pkg/cache"
	"meituan-ai-agent/pkg/database"
	"meituan-ai-agent/pkg/lock"
	"sync/atomic"
	"time"
)

type OrderService struct {
	requestCount int64
}

func NewOrderService() *OrderService {
	return &OrderService{}
}

func (s *OrderService) CreateOrder(ctx context.Context, userID, merchantID int64, dishes []string, totalPrice float64) (int64, error) {
	atomic.AddInt64(&s.requestCount, 1)
	
	lockKey := fmt.Sprintf("order:lock:%d:%d", userID, merchantID)
	distLock := lock.NewDistributedLock(
		cache.GetClient(),
		lockKey,
		"order-create",
		10*time.Second,
	)
	
	err := distLock.TryLockWithRetry(ctx, 3, 100*time.Millisecond)
	if err != nil {
		return 0, errors.New("too many concurrent orders")
	}
	defer distLock.Unlock(ctx)
	
	db := database.GetDB(userID)
	
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	
	var order struct {
		ID int64
	}
	
	err = tx.Exec(`
		INSERT INTO orders (user_id, merchant_id, dishes, total_price, status, order_time)
		VALUES (?, ?, ?, ?, 0, NOW())
	`, userID, merchantID, dishes, totalPrice).Error
	
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	
	err = tx.Raw("SELECT LAST_INSERT_ID() as id").Scan(&order).Error
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	
	return order.ID, nil
}

func (s *OrderService) GetRequestCount() int64 {
	return atomic.LoadInt64(&s.requestCount)
}

type OrderCache struct {
	orders map[int64]interface{}
}

func (s *OrderService) BatchCreateOrders(ctx context.Context, orders []OrderRequest) error {
	results := make(chan error, len(orders))
	semaphore := make(chan struct{}, 10)
	
	for _, order := range orders {
		semaphore <- struct{}{}
		
		go func(o OrderRequest) {
			defer func() { <-semaphore }()
			
			_, err := s.CreateOrder(ctx, o.UserID, o.MerchantID, o.Dishes, o.TotalPrice)
			results <- err
		}(order)
	}
	
	var firstErr error
	for i := 0; i < len(orders); i++ {
		if err := <-results; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	
	return firstErr
}

type OrderRequest struct {
	UserID     int64
	MerchantID int64
	Dishes     []string
	TotalPrice float64
}
