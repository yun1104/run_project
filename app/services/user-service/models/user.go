package models

import "time"

type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string    `gorm:"size:50;unique;not null" json:"username"`
	Password  string    `gorm:"size:100;not null" json:"-"`
	Phone     string    `gorm:"size:20;unique" json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

type UserPreference struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int64     `gorm:"index;not null" json:"user_id"`
	Categories   string    `gorm:"type:text" json:"categories"`
	PriceRange   string    `gorm:"size:50" json:"price_range"`
	Tastes       string    `gorm:"type:text" json:"tastes"`
	Merchants    string    `gorm:"type:text" json:"merchants"`
	DishKeywords string    `gorm:"type:text" json:"dish_keywords"`
	AvoidFoods   string    `gorm:"type:text" json:"avoid_foods"`
	OrderTimes   string    `gorm:"type:text" json:"order_times"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (UserPreference) TableName() string {
	return "user_preferences"
}

func GetPreferenceTableName(userID int64) string {
	return "user_preferences_" + string(rune(userID%100))
}
