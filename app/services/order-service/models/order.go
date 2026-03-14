package models

import "time"

type Order struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int64     `gorm:"index;not null" json:"user_id"`
	MerchantID   int64     `gorm:"index" json:"merchant_id"`
	MerchantName string    `gorm:"size:100" json:"merchant_name"`
	Dishes       string    `gorm:"type:text" json:"dishes"`
	TotalPrice   float64   `gorm:"type:decimal(10,2)" json:"total_price"`
	Status       int       `gorm:"default:0" json:"status"`
	OrderTime    time.Time `json:"order_time"`
	CreatedAt    time.Time `json:"created_at"`
}

func (Order) TableName() string {
	return "orders"
}

func GetOrderTableName(userID int64, orderTime time.Time) string {
	month := orderTime.Format("200601")
	return "orders_" + month
}
