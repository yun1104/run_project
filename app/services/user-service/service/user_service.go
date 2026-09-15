package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"xiangchisha/pkg/cache"
	"xiangchisha/pkg/database"
	"xiangchisha/services/user-service/models"
	"time"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) Register(username, password, phone string) error {
	db := database.GetDBByIndex(0)
	
	hashedPassword := hashPassword(password)
	user := &models.User{
		Username: username,
		Password: hashedPassword,
		Phone:    phone,
	}
	
	return db.Create(user).Error
}

func (s *UserService) Login(username, password string) (int64, error) {
	db := database.GetDBByIndex(0)
	
	var user models.User
	err := db.Where("username = ? AND password = ?", username, hashPassword(password)).First(&user).Error
	if err != nil {
		return 0, err
	}
	
	return user.ID, nil
}

func (s *UserService) GetPreference(ctx context.Context, userID int64) (*PreferenceData, error) {
	cacheKey := fmt.Sprintf("user:pref:%d", userID)
	
	var pref PreferenceData
	err := cache.Get(ctx, cacheKey, &pref)
	if err == nil {
		return &pref, nil
	}
	
	db := database.GetDB(userID)
	var dbPref models.UserPreference
	err = db.Table(models.GetPreferenceTableName(userID)).
		Where("user_id = ?", userID).First(&dbPref).Error
	if err != nil {
		return nil, err
	}
	
	pref = PreferenceData{
		UserID:       dbPref.UserID,
		Categories:   unmarshalStringArray(dbPref.Categories),
		PriceRange:   dbPref.PriceRange,
		Tastes:       unmarshalStringArray(dbPref.Tastes),
		Merchants:    unmarshalInt64Array(dbPref.Merchants),
		DishKeywords: unmarshalStringArray(dbPref.DishKeywords),
		AvoidFoods:   unmarshalStringArray(dbPref.AvoidFoods),
		OrderTimes:   unmarshalIntArray(dbPref.OrderTimes),
	}
	
	cache.Set(ctx, cacheKey, pref, time.Hour)
	
	return &pref, nil
}

func (s *UserService) UpdatePreference(ctx context.Context, pref *PreferenceData) error {
	db := database.GetDB(pref.UserID)
	
	dbPref := models.UserPreference{
		UserID:       pref.UserID,
		Categories:   marshalArray(pref.Categories),
		PriceRange:   pref.PriceRange,
		Tastes:       marshalArray(pref.Tastes),
		Merchants:    marshalArray(pref.Merchants),
		DishKeywords: marshalArray(pref.DishKeywords),
		AvoidFoods:   marshalArray(pref.AvoidFoods),
		OrderTimes:   marshalArray(pref.OrderTimes),
		UpdatedAt:    time.Now(),
	}
	
	err := db.Table(models.GetPreferenceTableName(pref.UserID)).
		Where("user_id = ?", pref.UserID).
		Updates(&dbPref).Error
	if err != nil {
		return err
	}
	
	cacheKey := fmt.Sprintf("user:pref:%d", pref.UserID)
	cache.Del(ctx, cacheKey)
	
	return nil
}

type PreferenceData struct {
	UserID       int64    `json:"user_id"`
	Categories   []string `json:"categories"`
	PriceRange   string   `json:"price_range"`
	Tastes       []string `json:"tastes"`
	Merchants    []int64  `json:"merchants"`
	DishKeywords []string `json:"dish_keywords"`
	AvoidFoods   []string `json:"avoid_foods"`
	OrderTimes   []int    `json:"order_times"`
}

func hashPassword(password string) string {
	hash := md5.Sum([]byte(password + "salt"))
	return hex.EncodeToString(hash[:])
}

func marshalArray(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func unmarshalStringArray(s string) []string {
	var arr []string
	json.Unmarshal([]byte(s), &arr)
	return arr
}

func unmarshalInt64Array(s string) []int64 {
	var arr []int64
	json.Unmarshal([]byte(s), &arr)
	return arr
}

func unmarshalIntArray(s string) []int {
	var arr []int
	json.Unmarshal([]byte(s), &arr)
	return arr
}
