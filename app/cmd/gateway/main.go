package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
	"xiangchisha/internal/distributed/apprpc"
	"xiangchisha/internal/distributed/contracts"
	"xiangchisha/internal/platform/config"
	"xiangchisha/internal/rpcjson"
	"xiangchisha/pkg/database"
	"xiangchisha/pkg/middleware"
	"xiangchisha/pkg/mq"
)

type locRecord struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Source    string  `json:"source"`
	UpdatedAt string  `json:"updated_at"`
}

type llmChatResultRaw struct {
	IsOrderIntent bool                     `json:"is_order_intent"`
	Reply         string                   `json:"reply"`
	Merchants     []map[string]interface{} `json:"merchants"`
}

type llmPythonInput struct {
	Requirement string                 `json:"requirement"`
	HasPref     bool                   `json:"has_pref"`
	Preference  map[string]interface{} `json:"preference"`
	Candidates  []contracts.Merchant   `json:"candidates"`
	NearbyFoods []amapNearbyFood       `json:"nearby_foods"`
}

type chatMessageRow struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       int64     `gorm:"column:user_id;index;not null"`
	SessionID    string    `gorm:"column:session_id;size:64;index;not null"`
	SessionTitle string    `gorm:"column:session_title;size:100;not null"`
	Role         string    `gorm:"column:role;size:20;not null"`
	Content      string    `gorm:"column:content;type:text;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (chatMessageRow) TableName() string { return "chat_messages" }

var locStore sync.Map
var chatStoreReady bool
var chatAsyncEnabled bool
var chatProducer *mq.Producer
var chatConsumers []*mq.Consumer

type chatAsyncMessage struct {
	UserID       int64  `json:"user_id"`
	SessionID    string `json:"session_id"`
	SessionTitle string `json:"session_title"`
	Role         string `json:"role"`
	Content      string `json:"content"`
	CreatedAt    string `json:"created_at"`
}

func main() {
	httpAddr := config.GetEnv("GATEWAY_HTTP_ADDR", "0.0.0.0:8080")
	appAddr := config.GetEnv("APP_GRPC_ADDR", "127.0.0.1:50050")
	initChatStore()
	initChatAsync()
	defer closeChatAsync()
	conn, err := grpc.Dial(appAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(rpcjson.Name)),
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	appClient := apprpc.NewClient(conn)

	r := gin.Default()
	r.Use(middleware.NewRateLimiter(200, time.Second).Middleware())
	r.Use(func(c *gin.Context) {
		path := strings.ToLower(strings.TrimSpace(c.Request.URL.Path))
		if path == "/" || path == "/account" || path == "/orders" ||
			strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".html") {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	})
	r.Static("/assets", "./web")
	r.GET("/", func(c *gin.Context) { c.File("./web/index.html") })
	r.GET("/account", func(c *gin.Context) { c.File("./web/account.html") })
	r.GET("/orders", func(c *gin.Context) { c.File("./web/orders.html") })

	api := r.Group("/api/v1")
	{
		user := api.Group("/user")
		{
			user.POST("/register", func(c *gin.Context) {
				var req contracts.RegisterRequest
				if c.ShouldBindJSON(&req) != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
					return
				}
				resp, err := appClient.Register(c.Request.Context(), &req)
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			user.POST("/login", func(c *gin.Context) {
				var req contracts.LoginRequest
				if c.ShouldBindJSON(&req) != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
					return
				}
				resp, err := appClient.Login(c.Request.Context(), &req)
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			user.GET("/me", authMiddleware(appClient), func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				resp, err := appClient.GetMe(c.Request.Context(), &contracts.UserIDRequest{UserID: userID})
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			user.GET("/location-permission", authMiddleware(appClient), func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				resp, err := appClient.GetLocationPermission(c.Request.Context(), &contracts.UserIDRequest{UserID: userID})
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			user.PUT("/location-permission", authMiddleware(appClient), func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				var req struct {
					LocationPermission string `json:"location_permission"`
				}
				if c.ShouldBindJSON(&req) != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
					return
				}
				resp, err := appClient.UpdateLocationPermission(c.Request.Context(), &contracts.UpdateLocationPermissionRequest{
					UserID:             userID,
					LocationPermission: req.LocationPermission,
				})
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			user.GET("/preference", authMiddleware(appClient), func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				resp, err := appClient.GetPreference(c.Request.Context(), &contracts.UserIDRequest{UserID: userID})
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			user.PUT("/preference", authMiddleware(appClient), func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				var req contracts.UpdatePreferenceRequest
				if c.ShouldBindJSON(&req) != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
					return
				}
				req.UserID = userID
				resp, err := appClient.UpdatePreference(c.Request.Context(), &req)
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			user.GET("/preference/questions", authMiddleware(appClient), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"code": 0,
					"message": "ok",
					"data": []gin.H{
						{"id": "spicy_level", "title": "你能接受的辣度？", "multi": false, "options": []string{"不辣", "微辣", "中辣", "特辣"}},
						{"id": "budget_range", "title": "单餐预算范围？", "multi": false, "options": []string{"20元以下", "20-40元", "40-80元", "80元以上"}},
						{"id": "cuisine_likes", "title": "喜欢的菜系？", "multi": true, "options": []string{"川菜", "粤菜", "湘菜", "日料", "韩餐", "西餐", "快餐"}},
						{"id": "avoid_foods", "title": "不吃的食物？", "multi": true, "options": []string{"海鲜", "牛肉", "羊肉", "猪肉", "鸡肉", "辣椒", "香菜"}},
						{"id": "diet_goal", "title": "饮食目标？", "multi": false, "options": []string{"减脂", "增肌", "保持", "随意"}},
						{"id": "dining_time", "title": "常用餐时间？", "multi": false, "options": []string{"早餐", "午餐", "晚餐", "夜宵", "不固定"}},
					},
				})
			})
			user.POST("/location", authMiddleware(appClient), func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				var req struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
					Source    string  `json:"source"`
				}
				if c.ShouldBindJSON(&req) != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
					return
				}
				if req.Source == "" {
					req.Source = "unknown"
				}
				rec := locRecord{Latitude: req.Latitude, Longitude: req.Longitude, Source: req.Source, UpdatedAt: time.Now().Format("2006-01-02 15:04:05")}
				locStore.Store(userID, rec)
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"location": rec}})
			})
			user.GET("/location/current", authMiddleware(appClient), func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				v, ok := locStore.Load(userID)
				if !ok {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "location not found, please authorize location first"})
					return
				}
				rec := v.(locRecord)
				withNearby := strings.EqualFold(strings.TrimSpace(c.Query("with_nearby")), "1") ||
					strings.EqualFold(strings.TrimSpace(c.Query("with_nearby")), "true")
				data := gin.H{"location": rec}
				if withNearby {
					radius := 3000
					if r, err := strconv.Atoi(strings.TrimSpace(c.Query("radius"))); err == nil && r > 0 {
						radius = r
					}
					limit := 8
					if l, err := strconv.Atoi(strings.TrimSpace(c.Query("limit"))); err == nil && l > 0 {
						limit = l
					}
					if limit > 20 {
						limit = 20
					}
					if nearby, err := fetchNearbyFoodsFromAmap(c.Request.Context(), rec.Latitude, rec.Longitude, radius, limit); err == nil {
						data["nearby_foods"] = nearby
					} else {
						log.Printf("query current location nearby failed: %v", err)
						data["nearby_foods"] = []amapNearbyFood{}
					}
				}
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
			})
			user.GET("/location/history", authMiddleware(appClient), func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				limit := 20
				if l, err := strconv.Atoi(strings.TrimSpace(c.Query("limit"))); err == nil && l > 0 && l <= 100 {
					limit = l
				}
				v, ok := locStore.Load(userID)
				history := []interface{}{}
				if ok {
					rec := v.(locRecord)
					history = []interface{}{gin.H{"latitude": rec.Latitude, "longitude": rec.Longitude, "source": rec.Source, "updated_at": rec.UpdatedAt}}
				}
				if len(history) > limit {
					history = history[:limit]
				}
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"history": history}})
			})
		}

		recommend := api.Group("/recommend", authMiddleware(appClient))
		{
			recommend.POST("/get", func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				var req contracts.RecommendRequest
				if c.ShouldBindJSON(&req) != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
					return
				}
				req.UserID = userID
				resp, err := appClient.GetRecommendations(c.Request.Context(), &req)
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": err.Error()})
					return
				}
				nearbyFoods := make([]amapNearbyFood, 0, 8)
				candidates := resp.Merchants
				if v, ok := locStore.Load(userID); ok {
					rec := v.(locRecord)
					if foods, ferr := fetchNearbyFoodsFromAmap(c.Request.Context(), rec.Latitude, rec.Longitude, 3000, 8); ferr == nil && len(foods) > 0 {
						nearbyFoods = foods
						candidates = mergeCandidatesWithNearby(foods)
					}
				}
				prefMap, hasPref := loadPreferenceForLLM(c.Request.Context(), appClient, userID)
				llmOut, llmErr := callPythonRecommend(c.Request.Context(), llmPythonInput{
					Requirement: req.Requirement,
					HasPref:     hasPref,
					Preference:  prefMap,
					Candidates:  candidates,
					NearbyFoods: nearbyFoods,
				})
				if llmErr != nil {
					if !isLikelyOrderIntent(req.Requirement) {
						c.JSON(http.StatusOK, gin.H{
							"code":            0,
							"message":         "ok",
							"reply":           "你好，我在。你可以告诉我预算、口味和送达时间，我再给你推荐外卖。",
							"is_order_intent": false,
							"merchants":       []contracts.Merchant{},
						})
						return
					}
					c.JSON(http.StatusOK, gin.H{
						"code":            resp.Code,
						"message":         resp.Message,
						"reply":           "已为你生成推荐结果。",
						"is_order_intent": true,
						"merchants":       candidates,
					})
					return
				}

				reply := strings.TrimSpace(llmOut.Reply)
				if reply == "" {
					if llmOut.IsOrderIntent {
						reply = "已为你生成推荐结果。"
					} else {
						reply = "你好，我在。"
					}
				}
				if !llmOut.IsOrderIntent {
					if isLikelyOrderIntent(req.Requirement) || len(candidates) == 0 {
						llmOut.IsOrderIntent = true
					} else {
						c.JSON(http.StatusOK, gin.H{
							"code":            0,
							"message":         "ok",
							"reply":           reply,
							"is_order_intent": false,
							"merchants":       []contracts.Merchant{},
						})
						return
					}
				}
				finalMerchants := pickMerchantsByLLM(candidates, llmOut.Merchants)
				if len(finalMerchants) == 0 {
					finalMerchants = candidates
				}
				if len(finalMerchants) == 0 {
					finalMerchants = []contracts.Merchant{
						{ID: 900001, Name: "香辣鸡腿饭", Category: "快餐", Rating: 4.7, AvgPrice: 28, Distance: "1.2km", DeliveryTime: 32, Tags: []string{"人气", "下饭"}, Reason: "预算匹配，口味偏辣"},
						{ID: 900002, Name: "鲜虾云吞面", Category: "面食", Rating: 4.8, AvgPrice: 30, Distance: "1.5km", DeliveryTime: 35, Tags: []string{"口碑", "清爽"}, Reason: "价格合适，配送稳定"},
					}
				}
				c.JSON(http.StatusOK, gin.H{
					"code":            0,
					"message":         "ok",
					"reply":           reply,
					"is_order_intent": true,
					"merchants":       finalMerchants,
				})
			})
		}
		chat := api.Group("/chat", authMiddleware(appClient))
		{
			chat.POST("/message", func(c *gin.Context) {
				if !chatStoreReady {
					c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
					return
				}
				userID := c.GetInt64("user_id")
				var req struct {
					SessionID    string `json:"session_id"`
					SessionTitle string `json:"session_title"`
					Role         string `json:"role"`
					Text         string `json:"text"`
				}
				if c.ShouldBindJSON(&req) != nil {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
					return
				}
				req.SessionID = strings.TrimSpace(req.SessionID)
				req.SessionTitle = strings.TrimSpace(req.SessionTitle)
				req.Role = strings.ToLower(strings.TrimSpace(req.Role))
				req.Text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(req.Text, "\r", " "), "\n", " "))
				if req.SessionID == "" || req.Text == "" {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request"})
					return
				}
				if req.Role != "user" && req.Role != "assistant" && req.Role != "system" {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid role"})
					return
				}
				if req.SessionTitle == "" {
					req.SessionTitle = "新会话"
				}
				row := chatMessageRow{
					UserID:       userID,
					SessionID:    req.SessionID,
					SessionTitle: req.SessionTitle,
					Role:         req.Role,
					Content:      req.Text,
					CreatedAt:    time.Now(),
				}

				if err := enqueueChatMessage(c.Request.Context(), row); err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "save chat failed"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
			})
			chat.GET("/history", func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				limit := 500
				if l, err := strconv.Atoi(strings.TrimSpace(c.Query("limit"))); err == nil && l > 0 && l <= 1000 {
					limit = l
				}
				if !chatStoreReady {
					c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"messages": []interface{}{}}})
					return
				}
				rows := make([]chatMessageRow, 0, limit)
				if err := database.GetDBByIndex(0).
					Where("user_id = ?", userID).
					Order("created_at asc, id asc").
					Limit(limit).
					Find(&rows).Error; err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "load chat failed"})
					return
				}
				msgs := make([]gin.H, 0, len(rows))
				for _, row := range rows {
					msgs = append(msgs, gin.H{
						"id":            row.ID,
						"session_id":    row.SessionID,
						"session_title": row.SessionTitle,
						"role":          row.Role,
						"text":          row.Content,
						"created_at":    row.CreatedAt.Format("2006-01-02 15:04:05"),
					})
				}
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"messages": msgs}})
			})
			chat.DELETE("/session/:session_id", func(c *gin.Context) {
				userID := c.GetInt64("user_id")
				sessionID := strings.TrimSpace(c.Param("session_id"))
				if sessionID == "" {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid session id"})
					return
				}
				if !chatStoreReady {
					c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
					return
				}
				if err := database.GetDBByIndex(0).
					Where("user_id = ? AND session_id = ?", userID, sessionID).
					Delete(&chatMessageRow{}).Error; err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "delete chat failed"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
			})
		}
	}

	_ = r.Run(httpAddr)
}

func authMiddleware(appClient *apprpc.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "missing token"})
			c.Abort()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		check, err := appClient.ValidateToken(ctx, &contracts.ValidateTokenRequest{Token: token})
		if err != nil || !check.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid token"})
			c.Abort()
			return
		}
		c.Set("user_id", check.UserID)
		c.Next()
	}
}

func initChatAsync() {
	if !chatStoreReady {
		chatAsyncEnabled = false
		return
	}

	brokers := config.SplitCSV(config.GetEnv("KAFKA_BROKERS", "127.0.0.1:9092"))
	if len(brokers) == 0 {
		chatAsyncEnabled = false
		return
	}
	topic := config.GetEnv("KAFKA_CHAT_TOPIC", "chat.message")
	groupID := config.GetEnv("KAFKA_CHAT_GROUP", "gateway-chat-writer")
	workers := config.GetEnvInt("KAFKA_CHAT_WORKERS", 4)
	if workers <= 0 {
		workers = 1
	}

	chatProducer = mq.NewProducer(brokers, topic)
	chatConsumers = make([]*mq.Consumer, 0, workers)
	chatAsyncEnabled = true

	for i := 0; i < workers; i++ {
		consumer := mq.NewConsumer(brokers, topic, groupID)
		chatConsumers = append(chatConsumers, consumer)
		go runChatConsumer(consumer)
	}
}

func closeChatAsync() {
	if chatProducer != nil {
		_ = chatProducer.Close()
	}
	for _, consumer := range chatConsumers {
		if consumer != nil {
			_ = consumer.Close()
		}
	}
}

func enqueueChatMessage(ctx context.Context, row chatMessageRow) error {
	if !chatAsyncEnabled || chatProducer == nil {
		return writeChatMessage(row)
	}

	msg := chatAsyncMessage{
		UserID:       row.UserID,
		SessionID:    row.SessionID,
		SessionTitle: row.SessionTitle,
		Role:         row.Role,
		Content:      row.Content,
		CreatedAt:    row.CreatedAt.Format(time.RFC3339Nano),
	}
	if err := chatProducer.Send(ctx, fmt.Sprintf("%d", row.UserID), msg); err != nil {
		return writeChatMessage(row)
	}
	return nil
}

func runChatConsumer(consumer *mq.Consumer) {
	for {
		msg, err := consumer.ReadMessage(context.Background())
		if err != nil {
			log.Printf("chat kafka consume failed: %v", err)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if err := persistChatMessage(msg.Value); err != nil {
			log.Printf("chat kafka persist failed: %v", err)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if err := consumer.Commit(context.Background(), msg); err != nil {
			log.Printf("chat kafka commit failed: %v", err)
			time.Sleep(300 * time.Millisecond)
		}
	}
}

func persistChatMessage(payload []byte) error {
	var m chatAsyncMessage
	if err := json.Unmarshal(payload, &m); err != nil {
		return err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, m.CreatedAt)
	if err != nil {
		createdAt = time.Now()
	}
	return writeChatMessage(chatMessageRow{
		UserID:       m.UserID,
		SessionID:    m.SessionID,
		SessionTitle: m.SessionTitle,
		Role:         m.Role,
		Content:      m.Content,
		CreatedAt:    createdAt,
	})
}

func writeChatMessage(row chatMessageRow) error {
	return database.GetDBByIndex(0).Create(&row).Error
}

func initChatStore() {
	mysqlHost := config.GetEnv("MYSQL_HOST", "127.0.0.1")
	mysqlPort := config.GetEnvInt("MYSQL_PORT", 3306)
	mysqlDB := config.GetEnv("MYSQL_DB", "meituan_db_0")
	db, err := initGatewayMySQLWithFallback(mysqlHost, mysqlPort, mysqlDB)
	if err != nil {
		log.Printf("gateway chat mysql disabled: %v", err)
		chatStoreReady = false
		return
	}
	if db == nil {
		chatStoreReady = false
		return
	}
	if err := db.AutoMigrate(&chatMessageRow{}); err != nil {
		log.Printf("gateway chat migrate failed: %v", err)
		chatStoreReady = false
		return
	}
	chatStoreReady = true
}

func initGatewayMySQLWithFallback(host string, port int, dbName string) (*gorm.DB, error) {
	user := config.GetEnv("MYSQL_USER", "app")
	password := config.GetEnv("MYSQL_PASSWORD", "123456")
	candidates := []database.Config{
		{Host: host, Port: port, User: user, Password: password, DBName: dbName, Charset: "utf8mb4"},
	}
	if _, ok := os.LookupEnv("MYSQL_USER"); !ok {
		candidates = append(candidates,
			database.Config{Host: host, Port: port, User: "root", Password: "123456", DBName: dbName, Charset: "utf8mb4"},
			database.Config{Host: host, Port: port, User: "root", Password: "root", DBName: dbName, Charset: "utf8mb4"},
		)
	}
	var lastErr error
	for _, cfg := range candidates {
		if err := database.InitMySQL([]database.Config{cfg}); err != nil {
			lastErr = err
			continue
		}
		db := database.GetDBByIndex(0)
		if db != nil {
			return db, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("mysql init failed with empty error")
	}
	return nil, lastErr
}

func loadPreferenceForLLM(ctx context.Context, appClient *apprpc.Client, userID int64) (map[string]interface{}, bool) {
	resp, err := appClient.GetPreference(ctx, &contracts.UserIDRequest{UserID: userID})
	if err != nil || resp == nil || resp.Code != 0 || !resp.Data.HasPreference || resp.Data.Preference == nil {
		return map[string]interface{}{}, false
	}
	p := resp.Data.Preference
	pref := map[string]interface{}{
		"categories":    p.Categories,
		"price_range":   p.PriceRange,
		"tastes":        p.Tastes,
		"dish_keywords": p.DishKeywords,
		"avoid_foods":   p.AvoidFoods,
	}
	return pref, true
}

func callPythonRecommend(ctx context.Context, input llmPythonInput) (llmChatResultRaw, error) {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return llmChatResultRaw{}, err
	}
	pythonCmd, pythonArgs := resolvePythonCommand()
	if pythonCmd == "" {
		return llmChatResultRaw{}, fmt.Errorf("python runtime not found")
	}
	cmd := exec.CommandContext(ctx, pythonCmd, pythonArgs...)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(inputBytes)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	output, err := cmd.Output()
	if err != nil {
		return llmChatResultRaw{}, err
	}
	var outRaw llmChatResultRaw
	if err := json.Unmarshal(output, &outRaw); err != nil {
		return llmChatResultRaw{}, err
	}
	return outRaw, nil
}

func resolvePythonCommand() (string, []string) {
	pyScript := "scripts/llm_recommend.py"
	if _, err := os.Stat(pyScript); err != nil {
		return "", nil
	}
	if _, err := exec.LookPath("python"); err == nil {
		return "python", []string{pyScript}
	}
	if _, err := exec.LookPath("py"); err == nil {
		return "py", []string{"-3", pyScript}
	}
	return "", nil
}

func pickMerchantsByLLM(candidates []contracts.Merchant, selected []map[string]interface{}) []contracts.Merchant {
	if len(selected) == 0 {
		return nil
	}
	idMap := make(map[int64]contracts.Merchant, len(candidates))
	for _, c := range candidates {
		idMap[c.ID] = c
	}
	out := make([]contracts.Merchant, 0, len(selected))
	for _, item := range selected {
		rawID, ok := item["id"]
		if !ok {
			continue
		}
		id, ok := parseMerchantID(rawID)
		if !ok {
			continue
		}
		base, ok := idMap[id]
		if !ok {
			continue
		}
		if reason, ok := item["reason"].(string); ok && strings.TrimSpace(reason) != "" {
			base.Reason = strings.TrimSpace(reason)
		}
		out = append(out, base)
	}
	return out
}

func parseMerchantID(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case string:
		id, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return 0, false
		}
		return id, true
	default:
		return 0, false
	}
}

func isLikelyOrderIntent(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	keywords := []string{
		"外卖", "点餐", "推荐", "吃什么", "想吃", "午饭", "晚饭", "夜宵", "早餐",
		"预算", "口味", "送达", "分钟", "辣", "清淡", "轻食", "川菜", "快餐",
		"takeaway", "order", "food", "meal", "lunch", "dinner", "delivery",
	}
	for _, k := range keywords {
		if strings.Contains(t, k) {
			return true
		}
	}
	return false
}
