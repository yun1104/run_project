package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"meituan-ai-agent/pkg/cache"
	"meituan-ai-agent/pkg/database"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	storeMu    sync.RWMutex
	orderSeq   int64 = 1000
	orderStore       = map[int64]Order{}
	sessions         = map[string]int64{}
	redisReady       = false
)

const prefCacheTTL = 24 * time.Hour
const sessionTTL = 72 * time.Hour

type Merchant struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	Rating       float64 `json:"rating"`
	AvgPrice     float64 `json:"avg_price"`
	DeliveryTime int     `json:"delivery_time"`
	Reason       string  `json:"reason"`
}

type Order struct {
	OrderID      int64     `json:"order_id"`
	UserID       int64     `json:"user_id"`
	MerchantID   int64     `json:"merchant_id"`
	MerchantName string    `json:"merchant_name"`
	Amount       float64   `json:"amount"`
	Status       string    `json:"status"`
	Paid         bool      `json:"paid"`
	CreatedAt    time.Time `json:"created_at"`
	PaidAt       time.Time `json:"paid_at,omitempty"`
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

type UserAccount struct {
	UserID       int64     `gorm:"column:id;primaryKey;autoIncrement" json:"user_id"`
	Username     string    `gorm:"column:username;uniqueIndex;size:50;not null" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;size:128;not null" json:"-"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
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
	
	api := r.Group("/api/v1")
	{
		user := api.Group("/user")
		{
			user.POST("/register", Register)
			user.POST("/login", Login)
			user.GET("/me", GetMe)
			user.GET("/preference", GetPreference)
			user.PUT("/preference", UpdatePreference)
			user.GET("/preference/questions", GetPreferenceQuestions)
		}
		
		order := api.Group("/order")
		{
			order.GET("/list", GetOrders)
			order.POST("/create", CreateOrder)
			order.GET("/detail", GetOrderDetail)
			order.POST("/auto-place-pay", AutoPlaceAndPay)
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
		"code":    0,
		"message": "success",
		"token":   token,
		"user_id": account.UserID,
		"username": account.Username,
	})
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

func GetOrders(c *gin.Context) {
	userID, ok := mustAuthUserID(c)
	if !ok {
		return
	}
	storeMu.RLock()
	defer storeMu.RUnlock()

	orders := make([]Order, 0, len(orderStore))
	for _, o := range orderStore {
		if o.UserID == userID {
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

	storeMu.RLock()
	order, ok := orderStore[orderID]
	storeMu.RUnlock()
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

func GetRecommendations(c *gin.Context) {
	if _, ok := mustAuthUserID(c); !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"merchants": recommendByRequirement("")}})
}

type ChatRequest struct {
	UserID      int64  `json:"user_id"`
	Requirement string `json:"requirement"`
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

	reply := "已收到你的需求，正在为你推荐外卖。"
	if strings.TrimSpace(req.Requirement) != "" {
		reply = "根据你的需求“" + req.Requirement + "”，为你推荐以下商家。"
	}
	req.UserID = authUserID
	if pref, ok := fetchPreference(c.Request.Context(), req.UserID); ok {
		reply += "（已结合你的偏好：" + pref.SpicyLevel + " / " + pref.BudgetRange + "）"
	}

	merchants := recommendByRequirement(req.Requirement)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"reply":     reply,
			"merchants": merchants,
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
	if err := db.AutoMigrate(&UserAccount{}, &UserPreferenceRow{}); err != nil {
		return err
	}
	return nil
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
		if err := cache.Get(ctx, fmt.Sprintf("user:acct:uname:%s", username), &acc); err == nil && acc.UserID > 0 {
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
