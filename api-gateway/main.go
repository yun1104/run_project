package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"xiangchisha/pkg/cache"
	"xiangchisha/pkg/database"
	"xiangchisha/services/meituan-crawler/crawler"
)

var (
	storeMu         sync.RWMutex
	orderSeq        int64 = 1000
	addressSeq      int64 = 10000
	orderStore            = map[int64]Order{}
	addressBook           = map[int64][]UserAddress{}
	sessions              = map[string]int64{}
	redisReady            = false
	meituanCrawler  *crawler.MeituanCrawler
)

const prefCacheTTL = 24 * time.Hour
const sessionTTL = 72 * time.Hour

type Merchant struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	Category          string   `json:"category"`
	Rating            float64  `json:"rating"`
	AvgPrice          float64  `json:"avg_price"`
	DeliveryTime      int      `json:"delivery_time"`
	Reason            string   `json:"reason"`
	RecommendedDishes []string `json:"recommended_dishes,omitempty"`
}

type Order struct {
	OrderID          int64       `json:"order_id"`
	UserID           int64       `json:"user_id"`
	MerchantID       int64       `json:"merchant_id"`
	MerchantName     string      `json:"merchant_name"`
	Items            []OrderItem `json:"items,omitempty"`
	DeliveryAddress  string      `json:"delivery_address,omitempty"`
	Remark           string      `json:"remark,omitempty"`
	EstimatedMinutes int         `json:"estimated_minutes,omitempty"`
	Amount           float64     `json:"amount"`
	Status           string      `json:"status"`
	Paid             bool        `json:"paid"`
	CreatedAt        time.Time   `json:"created_at"`
	PaidAt           time.Time   `json:"paid_at,omitempty"`
}

type OrderItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type UserPreference struct {
	UserID       int64    `json:"user_id"`
	SpicyLevel   string   `json:"spicy_level"`
	BudgetRange  string   `json:"budget_range"`
	CuisineLikes []string `json:"cuisine_likes"`
	AvoidFoods   []string `json:"avoid_foods"`
	DietGoal     string   `json:"diet_goal"`
	DiningTime   string   `json:"dining_time"`
	UpdatedAt    string   `json:"updated_at"`
}

type UserAddress struct {
	ID        int64     `json:"id"`
	Label     string    `json:"label"`
	Address   string    `json:"address"`
	Contact   string    `json:"contact,omitempty"`
	Phone     string    `json:"phone,omitempty"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

type UserAccount struct {
	UserID       int64     `gorm:"column:id;primaryKey;autoIncrement" json:"user_id"`
	Username     string    `gorm:"column:username;uniqueIndex;size:50;not null" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;size:128;not null" json:"-"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

type UserLocation struct {
	UserID    int64     `json:"user_id"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	IP        string    `json:"ip"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserAccount) TableName() string {
	return "users"
}

type UserPreferenceRow struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       int64     `gorm:"column:user_id;uniqueIndex;not null"`
	SpicyLevel   string    `gorm:"column:spicy_level;size:30"`
	BudgetRange  string    `gorm:"column:budget_range;size:30"`
	CuisineLikes string    `gorm:"column:cuisine_likes;type:text"`
	AvoidFoods   string    `gorm:"column:avoid_foods;type:text"`
	DietGoal     string    `gorm:"column:diet_goal;size:30"`
	DiningTime   string    `gorm:"column:dining_time;size:30"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (UserPreferenceRow) TableName() string {
	return "user_preferences"
}

type UserLocationRow struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex;not null"`
	Latitude  float64   `gorm:"column:latitude;type:decimal(10,6);not null"`
	Longitude float64   `gorm:"column:longitude;type:decimal(10,6);not null"`
	IP        string    `gorm:"column:ip;size:64"`
	Source    string    `gorm:"column:source;size:30"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (UserLocationRow) TableName() string {
	return "user_locations"
}

type UserLocationHistoryRow struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64     `gorm:"column:user_id;index;not null"`
	Latitude  float64   `gorm:"column:latitude;type:decimal(10,6);not null"`
	Longitude float64   `gorm:"column:longitude;type:decimal(10,6);not null"`
	IP        string    `gorm:"column:ip;size:64"`
	Source    string    `gorm:"column:source;size:30"`
	CreatedAt time.Time `gorm:"column:created_at;index"`
}

func (UserLocationHistoryRow) TableName() string {
	return "user_location_history"
}

func main() {
	httpAddr := getEnv("HTTP_ADDR", "0.0.0.0:8080")
	tcpAddr := getEnv("TCP_ADDR", "0.0.0.0:9091")
	udpAddr := getEnv("UDP_ADDR", "0.0.0.0:9092")
	allowedHost := strings.TrimSpace(getEnv("ALLOWED_HOST", ""))

	if err := initMySQLStorage(); err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	if err := initRedisCache(); err != nil {
		log.Printf("init redis failed, fallback db only: %v", err)
	}
	poolSize, _ := strconv.Atoi(getEnv("CRAWLER_POOL_SIZE", "1"))
	if poolSize <= 0 {
		poolSize = 1
	}
	if c, err := crawler.NewMeituanCrawler(poolSize); err == nil {
		meituanCrawler = c
		log.Printf("meituan crawler initialized, pool_size=%d", poolSize)
	} else {
		log.Printf("meituan crawler init skipped (playwright not available): %v", err)
	}

	go startTCPServer(tcpAddr)
	go startUDPServer(udpAddr)

	r := gin.Default()

	r.Use(RateLimitMiddleware())
	if allowedHost != "" {
		r.Use(HostLimitMiddleware(allowedHost))
	}
	r.Static("/assets", "./web")
	r.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})
	r.GET("/account", func(c *gin.Context) {
		c.File("./web/account.html")
	})
	r.GET("/orders", func(c *gin.Context) {
		c.File("./web/orders.html")
	})
	r.GET("/location", func(c *gin.Context) {
		c.File("./web/location.html")
	})

	api := r.Group("/api/v1")
	{
		user := api.Group("/user")
		{
			user.POST("/register", Register)
			user.POST("/login", Login)
			user.PUT("/password", ChangePassword)
			user.GET("/me", GetMe)
			user.GET("/preference", GetPreference)
			user.PUT("/preference", UpdatePreference)
			user.GET("/preference/questions", GetPreferenceQuestions)
			user.GET("/addresses", GetUserAddresses)
			user.POST("/addresses", SaveUserAddress)
			user.DELETE("/addresses/:id", DeleteUserAddress)
			user.POST("/location", SaveCurrentLocation)
			user.GET("/location/current", GetCurrentLocation)
			user.GET("/location/history", GetLocationHistory)
		}

		order := api.Group("/order")
		{
			order.GET("/list", GetOrders)
			order.POST("/create", CreateOrder)
			order.GET("/detail", GetOrderDetail)
			order.POST("/auto-place-pay", AutoPlaceAndPay)
			order.POST("/assistant-place", AssistantPlaceOrder)
			order.POST("/meituan-search", MeituanSearch)
			order.POST("/cancel", CancelOrder)
			order.POST("/reorder", ReorderOrder)
		}

		recommend := api.Group("/recommend")
		{
			recommend.POST("/get", GetRecommendations)
		}

		chat := api.Group("/chat")
		{
			chat.POST("/send", ChatSend)
		}

		network := api.Group("/network")
		{
			network.GET("/info", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"code": 0,
					"data": gin.H{
						"http_addr": httpAddr,
						"tcp_addr":  tcpAddr,
						"udp_addr":  udpAddr,
					},
				})
			})
		}
	}

	if err := r.Run(httpAddr); err != nil {
		log.Fatal(err)
	}
}

func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func HostLimitMiddleware(allowedHost string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}
		if !strings.EqualFold(host, allowedHost) && !strings.EqualFold(host, "localhost") && host != "127.0.0.1" {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "host not allowed"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func Register(c *gin.Context) {
	type registerRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "username/password invalid"})
		return
	}

	db := database.GetDBByIndex(0)
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "db not ready"})
		return
	}

	var exists UserAccount
	if err := db.Where("username = ?", req.Username).First(&exists).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "username already exists"})
		return
	}
	account := UserAccount{
		Username:     req.Username,
		PasswordHash: hashPassword(req.Password),
		CreatedAt:    time.Now(),
	}
	if err := db.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create user failed"})
		return
	}
	if redisReady {
		ctx := c.Request.Context()
		_ = cache.Set(ctx, fmt.Sprintf("user:acct:uname:%s", account.Username), account, prefCacheTTL)
		_ = cache.Set(ctx, fmt.Sprintf("user:acct:id:%d", account.UserID), account, prefCacheTTL)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"user_id": account.UserID}})
}

func Login(c *gin.Context) {
	type loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	account, ok := getUserByUsername(c.Request.Context(), strings.TrimSpace(req.Username))
	if !ok || account.PasswordHash != hashPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "username or password error"})
		return
	}
	token := fmt.Sprintf("u%d-%d", account.UserID, time.Now().UnixNano())
	storeMu.Lock()
	sessions[token] = account.UserID
	storeMu.Unlock()
	if redisReady {
		_ = cache.Set(c.Request.Context(), fmt.Sprintf("session:token:%s", token), account.UserID, sessionTTL)
	}
	c.JSON(http.StatusOK, gin.H{
		"code":     0,
		"message":  "success",
		"token":    token,
		"user_id":  account.UserID,
		"username": account.Username,
	})
}

func ChangePassword(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type changePasswordRequest struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	req.OldPassword = strings.TrimSpace(req.OldPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "new password too short"})
		return
	}

	db := database.GetDBByIndex(0)
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "db not ready"})
		return
	}

	var acc UserAccount
	if err := db.Where("id = ?", userID).First(&acc).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "user not found"})
		return
	}
	if acc.PasswordHash != hashPassword(req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "旧密码不对"})
		return
	}

	newHash := hashPassword(req.NewPassword)
	if newHash == acc.PasswordHash {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "new password must differ"})
		return
	}
	if err := db.Model(&UserAccount{}).Where("id = ?", userID).Update("password_hash", newHash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update password failed"})
		return
	}

	if redisReady {
		acc.PasswordHash = newHash
		ctx := c.Request.Context()
		_ = cache.Set(ctx, fmt.Sprintf("user:acct:id:%d", acc.UserID), acc, prefCacheTTL)
		_ = cache.Set(ctx, fmt.Sprintf("user:acct:uname:%s", acc.Username), acc, prefCacheTTL)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func GetMe(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	acc, ok := getUserByID(c.Request.Context(), userID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"user_id": acc.UserID, "username": acc.Username}})
}

func GetPreference(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}

	cacheKey := fmt.Sprintf("user:pref:%d", userID)
	var pref UserPreference
	if redisReady {
		if err := cache.Get(c.Request.Context(), cacheKey, &pref); err == nil && pref.UserID > 0 {
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": gin.H{
					"has_preference": true,
					"preference":     pref,
				},
			})
			return
		}
	}

	db := database.GetDBByIndex(0)
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "db not ready"})
		return
	}
	var row UserPreferenceRow
	if err := db.Where("user_id = ?", userID).First(&row).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"has_preference": false,
				"preference":     nil,
			},
		})
		return
	}
	pref = rowToPreference(row)
	if redisReady {
		_ = cache.Set(c.Request.Context(), cacheKey, pref, prefCacheTTL)
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"has_preference": true,
			"preference":     pref,
		},
	})
}

func UpdatePreference(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	var pref UserPreference
	if err := c.ShouldBindJSON(&pref); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid preference payload"})
		return
	}
	pref.UserID = userID
	pref.UpdatedAt = time.Now().Format(time.RFC3339)

	db := database.GetDBByIndex(0)
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "db not ready"})
		return
	}
	row := preferenceToRow(pref)
	var existing UserPreferenceRow
	err := db.Where("user_id = ?", pref.UserID).First(&existing).Error
	if err == nil {
		if err := db.Model(&existing).Updates(row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update preference failed"})
			return
		}
	} else {
		if err := db.Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create preference failed"})
			return
		}
	}
	if redisReady {
		cacheKey := fmt.Sprintf("user:pref:%d", pref.UserID)
		_ = cache.Set(c.Request.Context(), cacheKey, pref, prefCacheTTL)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": pref})
}

func GetPreferenceQuestions(c *gin.Context) {
	if _, ok := mustAuthUserID(c); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":      "spicy_level",
				"title":   "你能接受的辣度",
				"options": []string{"不辣", "微辣", "中辣", "重辣"},
				"multi":   false,
			},
			{
				"id":      "budget_range",
				"title":   "单餐预算",
				"options": []string{"20元以内", "20-40元", "40-60元", "60元以上"},
				"multi":   false,
			},
			{
				"id":      "cuisine_likes",
				"title":   "偏好菜系",
				"options": []string{"川菜", "粤菜", "轻食", "面食", "快餐", "日料"},
				"multi":   true,
			},
			{
				"id":      "avoid_foods",
				"title":   "忌口",
				"options": []string{"无", "海鲜", "牛羊肉", "乳制品", "花生"},
				"multi":   true,
			},
			{
				"id":      "diet_goal",
				"title":   "饮食目标",
				"options": []string{"随意吃", "减脂控卡", "增肌高蛋白", "清淡养胃"},
				"multi":   false,
			},
			{
				"id":      "dining_time",
				"title":   "常点时段",
				"options": []string{"早餐", "午餐", "晚餐", "夜宵"},
				"multi":   false,
			},
		},
	})
}

func GetUserAddresses(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	storeMu.RLock()
	list := append([]UserAddress{}, addressBook[userID]...)
	storeMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"addresses": list}})
}

func SaveUserAddress(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type saveAddressRequest struct {
		ID        int64  `json:"id"`
		Label     string `json:"label"`
		Address   string `json:"address"`
		Contact   string `json:"contact"`
		Phone     string `json:"phone"`
		IsDefault bool   `json:"is_default"`
	}
	var req saveAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	req.Address = strings.TrimSpace(req.Address)
	req.Contact = strings.TrimSpace(req.Contact)
	req.Phone = strings.TrimSpace(req.Phone)
	if req.Address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "address is required"})
		return
	}
	if req.Label == "" {
		req.Label = "常用地址"
	}

	storeMu.Lock()
	defer storeMu.Unlock()
	list := addressBook[userID]
	targetID := req.ID
	if req.ID > 0 {
		updated := false
		for i := range list {
			if list[i].ID == req.ID {
				list[i].Label = req.Label
				list[i].Address = req.Address
				list[i].Contact = req.Contact
				list[i].Phone = req.Phone
				list[i].IsDefault = req.IsDefault
				targetID = list[i].ID
				updated = true
				break
			}
		}
		if !updated {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "address not found"})
			return
		}
	} else {
		addressSeq++
		targetID = addressSeq
		list = append(list, UserAddress{
			ID:        targetID,
			Label:     req.Label,
			Address:   req.Address,
			Contact:   req.Contact,
			Phone:     req.Phone,
			IsDefault: req.IsDefault,
			CreatedAt: time.Now(),
		})
	}
	if req.IsDefault {
		for i := range list {
			list[i].IsDefault = list[i].ID == targetID
		}
	} else if len(list) > 0 {
		hasDefault := false
		for _, a := range list {
			if a.IsDefault {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			list[0].IsDefault = true
		}
	}
	addressBook[userID] = list
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"addresses": list}})
}

func DeleteUserAddress(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id"})
		return
	}
	storeMu.Lock()
	defer storeMu.Unlock()
	list := addressBook[userID]
	out := make([]UserAddress, 0, len(list))
	found := false
	for _, a := range list {
		if a.ID == id {
			found = true
			continue
		}
		out = append(out, a)
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "address not found"})
		return
	}
	if len(out) > 0 {
		hasDefault := false
		for _, a := range out {
			if a.IsDefault {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			out[0].IsDefault = true
		}
	}
	addressBook[userID] = out
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"addresses": out}})
}

func SaveCurrentLocation(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type saveLocationRequest struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Source    string  `json:"source"`
	}
	var req saveLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	if req.Latitude < -90 || req.Latitude > 90 || req.Longitude < -180 || req.Longitude > 180 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid latitude/longitude"})
		return
	}
	req.Source = strings.TrimSpace(req.Source)
	if req.Source == "" {
		req.Source = "unknown"
	}
	ip := extractClientIP(c)
	loc, err := upsertUserLocation(c.Request.Context(), userID, req.Latitude, req.Longitude, req.Source, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "save location failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"location": loc}})
}

func GetCurrentLocation(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	loc, found, err := fetchUserLocation(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query location failed"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "location not found, please authorize location first"})
		return
	}

	withNearby := strings.EqualFold(strings.TrimSpace(c.Query("with_nearby")), "1") ||
		strings.EqualFold(strings.TrimSpace(c.Query("with_nearby")), "true")
	respData := gin.H{"location": loc}
	if cityPinyin := fetchCityPinyinFromAmap(c.Request.Context(), loc.Latitude, loc.Longitude); cityPinyin != "" {
		respData["city_pinyin"] = cityPinyin
	}
	if withNearby {
		radius, _ := strconv.Atoi(strings.TrimSpace(c.Query("radius")))
		if radius <= 0 {
			radius = 3000
		}
		limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
		if limit <= 0 {
			limit = 8
		}
		if limit > 20 {
			limit = 20
		}
		if nearbyFoods, err := fetchNearbyFoodsFromAmap(c.Request.Context(), loc.Latitude, loc.Longitude, radius, limit); err == nil {
			respData["nearby_foods"] = nearbyFoods
		} else {
			log.Printf("query current location nearby failed: %v", err)
			respData["nearby_foods"] = []AmapNearbyFood{}
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": respData})
}

func GetLocationHistory(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	history, err := fetchUserLocationHistory(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query location history failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"history": history}})
}

func GetOrders(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	storeMu.Lock()
	defer storeMu.Unlock()
	now := time.Now()
	orders := make([]Order, 0, len(orderStore))
	for id, o := range orderStore {
		if o.UserID == userID {
			o = deriveOrderStatus(o, now)
			orderStore[id] = o
			orders = append(orders, o)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"orders": orders}})
}

func CreateOrder(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type createOrderRequest struct {
		MerchantID   int64   `json:"merchant_id"`
		MerchantName string  `json:"merchant_name"`
		Amount       float64 `json:"amount"`
	}

	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}

	storeMu.Lock()
	orderSeq++
	order := Order{
		OrderID:      orderSeq,
		UserID:       userID,
		MerchantID:   req.MerchantID,
		MerchantName: req.MerchantName,
		Amount:       req.Amount,
		Status:       "created",
		Paid:         false,
		CreatedAt:    time.Now(),
	}
	orderStore[order.OrderID] = order
	storeMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "order_id": order.OrderID})
}

func GetOrderDetail(c *gin.Context) {
	userID, okAuth := mustAuthUserID(c)
	if !okAuth {
		return
	}
	orderIDStr := c.Query("order_id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid order_id"})
		return
	}

	storeMu.Lock()
	order, ok := orderStore[orderID]
	if ok {
		order = deriveOrderStatus(order, time.Now())
		orderStore[orderID] = order
	}
	storeMu.Unlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "order not found"})
		return
	}
	if order.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": order})
}

func CancelOrder(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type cancelOrderRequest struct {
		OrderID int64 `json:"order_id"`
	}
	var req cancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid order_id"})
		return
	}
	storeMu.Lock()
	order, exists := orderStore[req.OrderID]
	if !exists {
		storeMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "order not found"})
		return
	}
	if order.UserID != userID {
		storeMu.Unlock()
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden"})
		return
	}
	order = deriveOrderStatus(order, time.Now())
	if order.Status == "delivered" {
		storeMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "delivered order cannot be cancelled"})
		return
	}
	order.Status = "cancelled"
	orderStore[req.OrderID] = order
	storeMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"order": order, "tip": "订单已取消"}})
}

func ReorderOrder(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type reorderRequest struct {
		OrderID         int64  `json:"order_id"`
		DeliveryAddress string `json:"delivery_address"`
		Remark          string `json:"remark"`
		AutoPay         bool   `json:"auto_pay"`
	}
	var req reorderRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid order_id"})
		return
	}

	storeMu.Lock()
	source, exists := orderStore[req.OrderID]
	if !exists {
		storeMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "source order not found"})
		return
	}
	if source.UserID != userID {
		storeMu.Unlock()
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden"})
		return
	}

	address := strings.TrimSpace(req.DeliveryAddress)
	if address == "" {
		address = strings.TrimSpace(source.DeliveryAddress)
	}
	if address == "" {
		for _, a := range addressBook[userID] {
			if a.IsDefault {
				address = strings.TrimSpace(a.Address)
				break
			}
		}
	}
	if address == "" {
		address = "请补充配送地址"
	}

	items := source.Items
	if len(items) == 0 {
		items = []OrderItem{{Name: "商家招牌套餐", Quantity: 1, Price: math.Max(12, source.Amount)}}
	}
	total := 0.0
	for i := range items {
		if items[i].Quantity <= 0 {
			items[i].Quantity = 1
		}
		if items[i].Price <= 0 {
			items[i].Price = 18
		}
		total += float64(items[i].Quantity) * items[i].Price
	}
	if total <= 0 {
		total = math.Max(12, source.Amount)
	}

	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = strings.TrimSpace(source.Remark)
	}
	if remark == "" {
		remark = "再来一单"
	}

	autoPay := req.AutoPay
	now := time.Now()
	status := "created"
	paid := false
	paidAt := time.Time{}
	if autoPay {
		status = "paid"
		paid = true
		paidAt = now
	}

	orderSeq++
	newOrder := Order{
		OrderID:          orderSeq,
		UserID:           userID,
		MerchantID:       source.MerchantID,
		MerchantName:     source.MerchantName,
		Items:            items,
		DeliveryAddress:  address,
		Remark:           remark,
		EstimatedMinutes: maxInt(18, source.EstimatedMinutes),
		Amount:           math.Round(total*100) / 100,
		Status:           status,
		Paid:             paid,
		CreatedAt:        now,
		PaidAt:           paidAt,
	}
	orderStore[newOrder.OrderID] = newOrder
	storeMu.Unlock()

	tip := fmt.Sprintf("已根据历史订单为你再次下单，订单号 %d。", newOrder.OrderID)
	if newOrder.Paid {
		tip += " 已自动支付。"
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"order": newOrder,
			"tip":   tip,
		},
	})
}

func deriveOrderStatus(order Order, now time.Time) Order {
	if order.Status == "cancelled" || order.Status == "delivered" {
		return order
	}
	elapsed := now.Sub(order.CreatedAt)
	switch {
	case elapsed >= 15*time.Minute:
		order.Status = "delivered"
	case elapsed >= 5*time.Minute:
		order.Status = "delivering"
	case elapsed >= 1*time.Minute:
		order.Status = "accepted"
	default:
		if order.Paid {
			order.Status = "paid"
		} else {
			order.Status = "created"
		}
	}
	return order
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func GetRecommendations(c *gin.Context) {
	if _, ok := mustAuthUserID(c); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"merchants": recommendByRequirement("")}})
}

type ChatRequest struct {
	UserID      int64   `json:"user_id"`
	Requirement string  `json:"requirement"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Radius      int     `json:"radius"`
}

type llmRecommendResult struct {
	Reply     string                 `json:"reply"`
	Merchants []llmRecommendMerchant `json:"merchants"`
}

type llmRecommendMerchant struct {
	ID     int64    `json:"id"`
	Reason string   `json:"reason"`
	Dishes []string `json:"dishes"`
}

type llmChatResultRaw struct {
	IsOrderIntent bool                     `json:"is_order_intent"`
	Reply         string                   `json:"reply"`
	Merchants     []map[string]interface{} `json:"merchants"`
}

type llmPythonInput struct {
	Requirement string           `json:"requirement"`
	HasPref     bool             `json:"has_pref"`
	Preference  UserPreference   `json:"preference"`
	Candidates  []Merchant       `json:"candidates"`
	NearbyFoods []AmapNearbyFood `json:"nearby_foods"`
}

func ChatSend(c *gin.Context) {
	authUserID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	req.Requirement = strings.TrimSpace(req.Requirement)

	reply := "已收到你的需求，正在为你推荐外卖。"
	if req.Requirement != "" {
		reply = "根据你的需求“" + req.Requirement + "”，为你推荐以下商家。"
	}
	req.UserID = authUserID
	var pref UserPreference
	hasPref := false
	if pref, ok := fetchPreference(c.Request.Context(), req.UserID); ok {
		reply += "（已结合你的偏好：" + pref.SpicyLevel + " / " + pref.BudgetRange + "）"
		hasPref = true
	}

	nearbyFoods := []AmapNearbyFood{}
	lat, lon := req.Latitude, req.Longitude
	if lat == 0 && lon == 0 {
		if saved, found, err := fetchUserLocation(c.Request.Context(), req.UserID); err == nil && found {
			lat, lon = saved.Latitude, saved.Longitude
		}
	}
	if lat != 0 && lon != 0 {
		if _, err := upsertUserLocation(c.Request.Context(), req.UserID, lat, lon, "chat_send", extractClientIP(c)); err != nil {
			log.Printf("save location from chat failed: %v", err)
		}
		radius := req.Radius
		if radius <= 0 {
			radius = 3000
		}
		if foods, err := fetchNearbyFoodsFromAmap(c.Request.Context(), lat, lon, radius, 8); err == nil {
			nearbyFoods = foods
			reply += fmt.Sprintf("（已获取附近%d家美食）", len(nearbyFoods))
		} else {
			log.Printf("amap nearby fallback: %v", err)
		}
	}

	// 只基于真实高德附近美食推荐，不瞎编
	var merchants []Merchant
	if len(nearbyFoods) > 0 {
		merchants = mergeCandidatesWithNearby(nil, nearbyFoods)
		if llmReply, llmMerchants, isOrder, err := recommendByModelScope(c.Request.Context(), req.Requirement, pref, hasPref, merchants, nearbyFoods); err == nil {
			if strings.TrimSpace(llmReply) != "" {
				reply = llmReply
			}
			if isOrder {
				if len(llmMerchants) > 0 {
					merchants = llmMerchants
				}
			} else {
				merchants = []Merchant{}
			}
		} else {
			log.Printf("modelscope fallback: %v", err)
		}
	} else {
		reply = "请先到定位页授权位置，以便推荐您附近的真实美食。"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"reply":        reply,
			"merchants":    merchants,
			"nearby_foods": nearbyFoods,
		},
	})
}

func recommendByRequirement(requirement string) []Merchant {
	text := strings.ToLower(strings.TrimSpace(requirement))
	if strings.Contains(text, "轻食") || strings.Contains(text, "减脂") {
		return []Merchant{
			{ID: 20001, Name: "轻食研究所", Category: "轻食", Rating: 4.8, AvgPrice: 38, DeliveryTime: 24, Reason: "低脂高蛋白，满足轻食需求"},
			{ID: 20002, Name: "谷物能量碗", Category: "轻食", Rating: 4.7, AvgPrice: 35, DeliveryTime: 26, Reason: "热量透明，适合控卡"},
		}
	}
	if strings.Contains(text, "辣") || strings.Contains(text, "川菜") {
		return []Merchant{
			{ID: 30001, Name: "老麻抄手·川味小馆", Category: "川菜", Rating: 4.8, AvgPrice: 36, DeliveryTime: 28, Reason: "口味偏辣，符合历史偏好"},
			{ID: 30002, Name: "辣子鸡饭堂", Category: "川湘菜", Rating: 4.7, AvgPrice: 32, DeliveryTime: 27, Reason: "麻辣稳定，价格友好"},
		}
	}
	return []Merchant{
		{ID: 10001, Name: "鲜香牛肉饭", Category: "快餐", Rating: 4.6, AvgPrice: 29, DeliveryTime: 22, Reason: "高复购商家，配送速度快"},
		{ID: 10002, Name: "家常木桶饭", Category: "快餐", Rating: 4.5, AvgPrice: 26, DeliveryTime: 25, Reason: "性价比高，出餐快"},
	}
}

func recommendByModelScope(ctx context.Context, requirement string, pref UserPreference, hasPref bool, candidates []Merchant, nearbyFoods []AmapNearbyFood) (string, []Merchant, bool, error) {
	pref = normalizePreferenceForLLM(pref)
	input := llmPythonInput{
		Requirement: requirement,
		HasPref:     hasPref,
		Preference:  pref,
		Candidates:  candidates,
		NearbyFoods: nearbyFoods,
	}

	if outRaw, err := callPythonRecommendService(ctx, input); err == nil {
		return mapLLMOutputToMerchants(outRaw, candidates)
	} else {
		log.Printf("python recommend fallback: %v", err)
	}
	inputBytes, _ := json.Marshal(input)

	pythonCmd, pythonArgs := resolvePythonCommand()
	if pythonCmd == "" {
		return "", nil, false, fmt.Errorf("python runtime not found")
	}
	cmd := exec.CommandContext(ctx, pythonCmd, pythonArgs...)
	cmd.Dir = "."
	cmd.Stdin = strings.NewReader(string(inputBytes))
	output, err := cmd.Output()
	if err != nil {
		return "", nil, false, err
	}

	var outRaw llmChatResultRaw
	if err := json.Unmarshal(output, &outRaw); err != nil {
		return "", nil, false, err
	}
	return mapLLMOutputToMerchants(outRaw, candidates)
}

func mapLLMOutputToMerchants(outRaw llmChatResultRaw, candidates []Merchant) (string, []Merchant, bool, error) {
	if !outRaw.IsOrderIntent {
		return outRaw.Reply, []Merchant{}, false, nil
	}
	if len(outRaw.Merchants) == 0 {
		return outRaw.Reply, nil, true, nil
	}

	idMap := map[int64]Merchant{}
	for _, c := range candidates {
		idMap[c.ID] = c
	}
	finalMerchants := make([]Merchant, 0, len(outRaw.Merchants))
	for _, item := range outRaw.Merchants {
		rawID, okID := item["id"]
		if !okID {
			continue
		}
		var id int64
		switch v := rawID.(type) {
		case float64:
			id = int64(v)
		case string:
			id, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		default:
			continue
		}
		if base, ok := idMap[id]; ok {
			reason, _ := item["reason"].(string)
			m := llmRecommendMerchant{ID: id, Reason: reason, Dishes: parseStringSlice(item["dishes"])}
			if strings.TrimSpace(m.Reason) != "" {
				base.Reason = m.Reason
			}
			if len(m.Dishes) > 0 {
				base.RecommendedDishes = m.Dishes
			}
			finalMerchants = append(finalMerchants, base)
		}
	}
	return outRaw.Reply, finalMerchants, true, nil
}

func normalizePreferenceForLLM(pref UserPreference) UserPreference {
	if pref.CuisineLikes == nil {
		pref.CuisineLikes = []string{}
	}
	if pref.AvoidFoods == nil {
		pref.AvoidFoods = []string{}
	}
	return pref
}

func parseStringSlice(v interface{}) []string {
	switch raw := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(raw))
		for _, s := range raw {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func resolvePythonCommand() (string, []string) {
	if _, err := exec.LookPath("python"); err == nil {
		return "python", []string{"scripts/llm_recommend.py"}
	}
	if _, err := exec.LookPath("py"); err == nil {
		return "py", []string{"-3", "scripts/llm_recommend.py"}
	}
	return "", nil
}

func AutoPlaceAndPay(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type autoPayRequest struct {
		MerchantID   int64   `json:"merchant_id"`
		MerchantName string  `json:"merchant_name"`
		Amount       float64 `json:"amount"`
	}

	var req autoPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	if req.MerchantID <= 0 || req.Amount <= 0 || strings.TrimSpace(req.MerchantName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid order fields"})
		return
	}

	storeMu.Lock()
	orderSeq++
	now := time.Now()
	order := Order{
		OrderID:      orderSeq,
		UserID:       userID,
		MerchantID:   req.MerchantID,
		MerchantName: req.MerchantName,
		Amount:       req.Amount,
		Status:       "paid",
		Paid:         true,
		CreatedAt:    now,
		PaidAt:       now,
	}
	orderStore[order.OrderID] = order
	storeMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"order_id": order.OrderID,
			"status":   order.Status,
			"paid":     order.Paid,
			"tip":      fmt.Sprintf("订单 %d 已自动下单并支付", order.OrderID),
		},
	})
}

func AssistantPlaceOrder(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type placeOrderRequest struct {
		MerchantID      int64       `json:"merchant_id"`
		MerchantName    string      `json:"merchant_name"`
		Items           []OrderItem `json:"items"`
		DeliveryAddress string      `json:"delivery_address"`
		Remark          string      `json:"remark"`
		AutoPay         bool        `json:"auto_pay"`
	}

	var req placeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	req.MerchantName = strings.TrimSpace(req.MerchantName)
	req.DeliveryAddress = strings.TrimSpace(req.DeliveryAddress)
	req.Remark = strings.TrimSpace(req.Remark)
	if req.MerchantID <= 0 || req.MerchantName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "merchant is required"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "items are required"})
		return
	}

	cleanItems := make([]OrderItem, 0, len(req.Items))
	var total float64
	for _, item := range req.Items {
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			continue
		}
		if item.Quantity <= 0 {
			item.Quantity = 1
		}
		if item.Price <= 0 {
			item.Price = 18
		}
		total += float64(item.Quantity) * item.Price
		cleanItems = append(cleanItems, item)
	}
	if len(cleanItems) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "valid items are required"})
		return
	}
	if total <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid amount"})
		return
	}

	eta := 25
	now := time.Now()
	status := "created"
	paid := false
	paidAt := time.Time{}
	if req.AutoPay {
		status = "paid"
		paid = true
		paidAt = now
	}

	storeMu.Lock()
	orderSeq++
	order := Order{
		OrderID:          orderSeq,
		UserID:           userID,
		MerchantID:       req.MerchantID,
		MerchantName:     req.MerchantName,
		Items:            cleanItems,
		DeliveryAddress:  req.DeliveryAddress,
		Remark:           req.Remark,
		EstimatedMinutes: eta,
		Amount:           math.Round(total*100) / 100,
		Status:           status,
		Paid:             paid,
		CreatedAt:        now,
		PaidAt:           paidAt,
	}
	orderStore[order.OrderID] = order
	storeMu.Unlock()

	tip := fmt.Sprintf("已为你在 %s 下单，预计 %d 分钟送达。", order.MerchantName, order.EstimatedMinutes)
	if order.Paid {
		tip = fmt.Sprintf("%s 订单已自动支付。", tip)
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"order": order,
			"tip":   tip,
		},
	})
}

func MeituanSearch(c *gin.Context) {
	_, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	type req struct {
		MerchantName string `json:"merchant_name"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
		return
	}
	merchantName := strings.TrimSpace(r.MerchantName)
	if merchantName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "merchant_name is required"})
		return
	}
	if meituanCrawler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "美团搜索服务暂不可用，请稍后再试",
			"data":    gin.H{"found": false},
		})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	storeURL, searchURL, found, err := meituanCrawler.SearchStore(ctx, merchantName)
	if err != nil {
		log.Printf("meituan search error: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    gin.H{"found": false},
		})
		return
	}
	if found {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    gin.H{"found": true, "store_url": storeURL},
		})
	} else {
		respData := gin.H{"found": false}
		if searchURL != "" {
			respData["search_url"] = searchURL
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    respData,
		})
	}
}

func startTCPServer(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("tcp listen failed: %v", err)
		return
	}
	log.Printf("tcp server started at %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleTCPConn(conn)
	}
}

func handleTCPConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	msg := strings.TrimSpace(string(buf[:n]))
	if strings.EqualFold(msg, "PING") {
		_, _ = conn.Write([]byte("PONG\n"))
		return
	}
	merchants := recommendByRequirement(msg)
	data, _ := json.Marshal(merchants)
	_, _ = conn.Write(append(data, '\n'))
}

func startUDPServer(addr string) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Printf("udp resolve failed: %v", err)
		return
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Printf("udp listen failed: %v", err)
		return
	}
	log.Printf("udp server started at %s", addr)
	defer conn.Close()

	buf := make([]byte, 2048)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		msg := strings.TrimSpace(string(buf[:n]))
		if strings.EqualFold(msg, "PING") {
			_, _ = conn.WriteToUDP([]byte("PONG"), remote)
			continue
		}
		merchants := recommendByRequirement(msg)
		data, _ := json.Marshal(merchants)
		_, _ = conn.WriteToUDP(data, remote)
	}
}

func getEnv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func mustAuthUserID(c *gin.Context) (int64, bool) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if auth == "" || !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "missing token"})
		return 0, false
	}
	token := strings.TrimSpace(auth[7:])
	if redisReady {
		var uid int64
		if err := cache.Get(c.Request.Context(), fmt.Sprintf("session:token:%s", token), &uid); err == nil && uid > 0 {
			return uid, true
		}
	}
	storeMu.RLock()
	userID, ok := sessions[token]
	storeMu.RUnlock()
	if !ok || userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid token"})
		return 0, false
	}
	return userID, true
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password + "::mt-agent"))
	return hex.EncodeToString(sum[:])
}

func initMySQLStorage() error {
	port, _ := strconv.Atoi(getEnv("MYSQL_PORT", "3306"))
	cfg := database.Config{
		Host:     getEnv("MYSQL_HOST", "127.0.0.1"),
		Port:     port,
		User:     getEnv("MYSQL_USER", "root"),
		Password: getEnv("MYSQL_PASSWORD", "root"),
		DBName:   getEnv("MYSQL_DB", "meituan_db_0"),
		Charset:  getEnv("MYSQL_CHARSET", "utf8mb4"),
	}
	if err := database.InitMySQL([]database.Config{cfg}); err != nil {
		return err
	}
	db := database.GetDBByIndex(0)
	if db == nil {
		return fmt.Errorf("mysql db nil")
	}
	if err := db.AutoMigrate(&UserAccount{}, &UserPreferenceRow{}, &UserLocationRow{}, &UserLocationHistoryRow{}); err != nil {
		return err
	}
	return nil
}

func upsertUserLocation(ctx context.Context, userID int64, latitude float64, longitude float64, source string, ip string) (UserLocation, error) {
	db := database.GetDBByIndex(0)
	if db == nil {
		return UserLocation{}, fmt.Errorf("db not ready")
	}
	now := time.Now()
	var row UserLocationRow
	if err := db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error; err == nil {
		row.Latitude = latitude
		row.Longitude = longitude
		row.IP = ip
		row.Source = source
		row.UpdatedAt = now
		if err := db.WithContext(ctx).Save(&row).Error; err != nil {
			return UserLocation{}, err
		}
		if err := appendUserLocationHistory(ctx, userID, latitude, longitude, source, ip); err != nil {
			log.Printf("append location history failed: %v", err)
		}
		return UserLocation{
			UserID:    userID,
			Latitude:  row.Latitude,
			Longitude: row.Longitude,
			IP:        row.IP,
			Source:    row.Source,
			UpdatedAt: row.UpdatedAt,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return UserLocation{}, err
	}
	row = UserLocationRow{
		UserID:    userID,
		Latitude:  latitude,
		Longitude: longitude,
		IP:        ip,
		Source:    source,
		UpdatedAt: now,
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return UserLocation{}, err
	}
	if err := appendUserLocationHistory(ctx, userID, latitude, longitude, source, ip); err != nil {
		log.Printf("append location history failed: %v", err)
	}
	return UserLocation{
		UserID:    userID,
		Latitude:  row.Latitude,
		Longitude: row.Longitude,
		IP:        row.IP,
		Source:    row.Source,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func fetchUserLocation(ctx context.Context, userID int64) (UserLocation, bool, error) {
	db := database.GetDBByIndex(0)
	if db == nil {
		return UserLocation{}, false, fmt.Errorf("db not ready")
	}
	var row UserLocationRow
	if err := db.WithContext(ctx).Where("user_id = ?", userID).First(&row).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return UserLocation{}, false, err
		}
		return UserLocation{}, false, nil
	}
	return UserLocation{
		UserID:    row.UserID,
		Latitude:  row.Latitude,
		Longitude: row.Longitude,
		IP:        row.IP,
		Source:    row.Source,
		UpdatedAt: row.UpdatedAt,
	}, true, nil
}

func appendUserLocationHistory(ctx context.Context, userID int64, latitude float64, longitude float64, source string, ip string) error {
	db := database.GetDBByIndex(0)
	if db == nil {
		return fmt.Errorf("db not ready")
	}
	row := UserLocationHistoryRow{
		UserID:    userID,
		Latitude:  latitude,
		Longitude: longitude,
		IP:        ip,
		Source:    source,
		CreatedAt: time.Now(),
	}
	return db.WithContext(ctx).Create(&row).Error
}

func fetchUserLocationHistory(ctx context.Context, userID int64, limit int) ([]UserLocation, error) {
	db := database.GetDBByIndex(0)
	if db == nil {
		return nil, fmt.Errorf("db not ready")
	}
	rows := make([]UserLocationHistoryRow, 0, limit)
	if err := db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]UserLocation, 0, len(rows))
	for _, r := range rows {
		out = append(out, UserLocation{
			UserID:    r.UserID,
			Latitude:  r.Latitude,
			Longitude: r.Longitude,
			IP:        r.IP,
			Source:    r.Source,
			UpdatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func extractClientIP(c *gin.Context) string {
	pickFirst := func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return ""
		}
		if strings.Contains(v, ",") {
			parts := strings.Split(v, ",")
			return strings.TrimSpace(parts[0])
		}
		return v
	}
	candidates := []string{
		pickFirst(c.GetHeader("X-Forwarded-For")),
		strings.TrimSpace(c.GetHeader("X-Real-IP")),
		strings.TrimSpace(c.ClientIP()),
	}
	for _, ip := range candidates {
		if ip == "" {
			continue
		}
		if host, _, err := net.SplitHostPort(ip); err == nil && host != "" {
			ip = host
		}
		ip = strings.Trim(ip, "[]")
		if ip != "" && !strings.EqualFold(ip, "unknown") {
			return ip
		}
	}
	return ""
}

func initRedisCache() error {
	addrs := strings.Split(getEnv("REDIS_ADDRS", "127.0.0.1:6379"), ",")
	password := getEnv("REDIS_PASSWORD", "")
	if err := cache.InitRedis(addrs, password); err != nil {
		return err
	}
	redisReady = true
	return nil
}

func preferenceToRow(pref UserPreference) UserPreferenceRow {
	cl, _ := json.Marshal(pref.CuisineLikes)
	af, _ := json.Marshal(pref.AvoidFoods)
	t, _ := time.Parse(time.RFC3339, pref.UpdatedAt)
	if t.IsZero() {
		t = time.Now()
	}
	return UserPreferenceRow{
		UserID:       pref.UserID,
		SpicyLevel:   pref.SpicyLevel,
		BudgetRange:  pref.BudgetRange,
		CuisineLikes: string(cl),
		AvoidFoods:   string(af),
		DietGoal:     pref.DietGoal,
		DiningTime:   pref.DiningTime,
		UpdatedAt:    t,
	}
}

func rowToPreference(row UserPreferenceRow) UserPreference {
	var cuisine []string
	var avoid []string
	_ = json.Unmarshal([]byte(row.CuisineLikes), &cuisine)
	_ = json.Unmarshal([]byte(row.AvoidFoods), &avoid)
	return UserPreference{
		UserID:       row.UserID,
		SpicyLevel:   row.SpicyLevel,
		BudgetRange:  row.BudgetRange,
		CuisineLikes: cuisine,
		AvoidFoods:   avoid,
		DietGoal:     row.DietGoal,
		DiningTime:   row.DiningTime,
		UpdatedAt:    row.UpdatedAt.Format(time.RFC3339),
	}
}

func fetchPreference(ctx context.Context, userID int64) (UserPreference, bool) {
	cacheKey := fmt.Sprintf("user:pref:%d", userID)
	var pref UserPreference
	if redisReady {
		if err := cache.Get(ctx, cacheKey, &pref); err == nil && pref.UserID > 0 {
			return pref, true
		}
	}
	db := database.GetDBByIndex(0)
	if db == nil {
		return UserPreference{}, false
	}
	var row UserPreferenceRow
	if err := db.Where("user_id = ?", userID).First(&row).Error; err != nil {
		return UserPreference{}, false
	}
	pref = rowToPreference(row)
	if redisReady {
		_ = cache.Set(ctx, cacheKey, pref, prefCacheTTL)
	}
	return pref, true
}

func getUserByUsername(ctx context.Context, username string) (UserAccount, bool) {
	if redisReady {
		var acc UserAccount
		if err := cache.Get(ctx, fmt.Sprintf("user:acct:uname:%s", username), &acc); err == nil && acc.UserID > 0 && acc.PasswordHash != "" {
			return acc, true
		}
	}
	db := database.GetDBByIndex(0)
	if db == nil {
		return UserAccount{}, false
	}
	var acc UserAccount
	if err := db.Where("username = ?", username).First(&acc).Error; err != nil {
		return UserAccount{}, false
	}
	if redisReady {
		_ = cache.Set(ctx, fmt.Sprintf("user:acct:uname:%s", username), acc, prefCacheTTL)
		_ = cache.Set(ctx, fmt.Sprintf("user:acct:id:%d", acc.UserID), acc, prefCacheTTL)
	}
	return acc, true
}

func getUserByID(ctx context.Context, userID int64) (UserAccount, bool) {
	if redisReady {
		var acc UserAccount
		if err := cache.Get(ctx, fmt.Sprintf("user:acct:id:%d", userID), &acc); err == nil && acc.UserID > 0 {
			return acc, true
		}
	}
	db := database.GetDBByIndex(0)
	if db == nil {
		return UserAccount{}, false
	}
	var acc UserAccount
	if err := db.Where("id = ?", userID).First(&acc).Error; err != nil {
		return UserAccount{}, false
	}
	if redisReady {
		_ = cache.Set(ctx, fmt.Sprintf("user:acct:id:%d", userID), acc, prefCacheTTL)
		_ = cache.Set(ctx, fmt.Sprintf("user:acct:uname:%s", acc.Username), acc, prefCacheTTL)
	}
	return acc, true
}
