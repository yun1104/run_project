package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"xiangchisha/internal/distributed/contracts"
	"xiangchisha/internal/platform/cachex"
	"xiangchisha/internal/platform/config"
	"xiangchisha/pkg/cache"
	"xiangchisha/pkg/database"
	"xiangchisha/pkg/middleware"
)

type RecommendService struct {
	breaker *middleware.CircuitBreaker
	db      *gorm.DB
}

func NewRecommendService() *RecommendService {
	s := &RecommendService{
		breaker: middleware.NewCircuitBreaker(),
	}
	s.initStorage()
	return s
}

func (s *RecommendService) GetRecommendations(ctx context.Context, req *contracts.RecommendRequest) (*contracts.RecommendResponse, error) {
	cacheKey := fmt.Sprintf("recommend:%d:%s:%s", req.UserID, strings.TrimSpace(req.Requirement), strings.TrimSpace(req.Location))
	var redisMerchants []contracts.Merchant
	if err := cache.Get(ctx, cacheKey, &redisMerchants); err == nil {
		return &contracts.RecommendResponse{Code: 0, Message: "ok", Merchants: redisMerchants}, nil
	}
	if merchants, ok := s.getSnapshotFromMySQL(cacheKey); ok {
		_ = cachex.SetWithJitter(ctx, cacheKey, merchants, 10*time.Minute, 90)
		return &contracts.RecommendResponse{Code: 0, Message: "ok", Merchants: merchants}, nil
	}

	var (
		mu       sync.Mutex
		recalled []contracts.Merchant
		wg       sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		items := s.preferenceRecall(ctx, req.UserID)
		mu.Lock()
		recalled = append(recalled, items...)
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		items := s.hotRecall(ctx, req.Location)
		mu.Lock()
		recalled = append(recalled, items...)
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		items := s.cfRecall(ctx, req.UserID)
		mu.Lock()
		recalled = append(recalled, items...)
		mu.Unlock()
	}()
	wg.Wait()

	merged := dedupeMerchants(recalled)
	_ = s.breaker.Execute(func() error {
		s.rank(merged, req.Requirement)
		return nil
	})
	if len(merged) > 20 {
		merged = merged[:20]
	}
	if err := s.saveSnapshotToMySQL(cacheKey, merged); err == nil {
		_ = cache.Del(ctx, cacheKey)
	}
	return &contracts.RecommendResponse{Code: 0, Message: "ok", Merchants: merged}, nil
}

func (s *RecommendService) preferenceRecall(_ context.Context, userID int64) []contracts.Merchant {
	return []contracts.Merchant{
		{ID: 1001, Name: "香辣鸡腿饭", Category: "快餐", Rating: 4.8, AvgPrice: 28, Distance: "1.1km", DeliveryTime: 30, Tags: []string{"高复购", "口味稳定"}, Reason: fmt.Sprintf("命中偏好 user=%d", userID)},
	}
}

func (s *RecommendService) hotRecall(_ context.Context, location string) []contracts.Merchant {
	if location == "" {
		location = "default"
	}
	return []contracts.Merchant{
		{ID: 1002, Name: "鲜虾云吞面", Category: "面食", Rating: 4.7, AvgPrice: 24, Distance: "0.9km", DeliveryTime: 25, Tags: []string{"热门", "好评高"}, Reason: "热门榜单召回@" + location},
	}
}

func (s *RecommendService) cfRecall(_ context.Context, userID int64) []contracts.Merchant {
	return []contracts.Merchant{
		{ID: 1003, Name: "轻食能量碗", Category: "轻食", Rating: 4.6, AvgPrice: 35, Distance: "2.2km", DeliveryTime: 35, Tags: []string{"协同过滤", "同类用户喜欢"}, Reason: fmt.Sprintf("cf召回 user=%d", userID)},
	}
}

func (s *RecommendService) rank(merchants []contracts.Merchant, requirement string) {
	req := strings.ToLower(strings.TrimSpace(requirement))
	for i := range merchants {
		base := merchants[i].Rating*20 - merchants[i].AvgPrice*0.2 - float64(i)
		if req != "" && strings.Contains(strings.ToLower(merchants[i].Category), req) {
			base += 10
		}
		merchants[i].Score = base
	}
	sort.SliceStable(merchants, func(i, j int) bool { return merchants[i].Score > merchants[j].Score })
}

func dedupeMerchants(in []contracts.Merchant) []contracts.Merchant {
	seen := make(map[int64]struct{}, len(in))
	out := make([]contracts.Merchant, 0, len(in))
	for _, m := range in {
		if _, ok := seen[m.ID]; ok {
			continue
		}
		seen[m.ID] = struct{}{}
		out = append(out, m)
	}
	return out
}

type recommendSnapshotRow struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	CacheKey      string    `gorm:"column:cache_key;size:255;uniqueIndex;not null"`
	MerchantsJSON string    `gorm:"column:merchants_json;type:longtext"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (recommendSnapshotRow) TableName() string { return "recommendation_snapshots" }

func (s *RecommendService) getSnapshotFromMySQL(key string) ([]contracts.Merchant, bool) {
	if s.db == nil {
		return nil, false
	}
	var row recommendSnapshotRow
	if err := s.db.Where("cache_key = ?", key).First(&row).Error; err != nil {
		return nil, false
	}
	var merchants []contracts.Merchant
	if err := json.Unmarshal([]byte(row.MerchantsJSON), &merchants); err != nil {
		return nil, false
	}
	return merchants, true
}

func (s *RecommendService) saveSnapshotToMySQL(key string, merchants []contracts.Merchant) error {
	if s.db == nil {
		return nil
	}
	b, err := json.Marshal(merchants)
	if err != nil {
		return err
	}
	row := recommendSnapshotRow{
		CacheKey:      key,
		MerchantsJSON: string(b),
		UpdatedAt:     time.Now(),
	}
	return s.db.Where("cache_key = ?", key).Assign(&row).FirstOrCreate(&recommendSnapshotRow{}).Error
}

func (s *RecommendService) initStorage() {
	mysqlHost := config.GetEnv("MYSQL_HOST", "127.0.0.1")
	mysqlPort := config.GetEnvInt("MYSQL_PORT", 3306)
	mysqlDB := config.GetEnv("MYSQL_DB", "meituan_db_0")
	db, err := initRecommendMySQLWithFallback(mysqlHost, mysqlPort, mysqlDB)
	if err != nil {
		log.Printf("recommend-service mysql disabled: %v", err)
	} else {
		s.db = db
		if s.db != nil {
			_ = s.db.AutoMigrate(&recommendSnapshotRow{})
		}
	}
	redisAddrs := config.SplitCSV(config.GetEnv("REDIS_ADDRS", "127.0.0.1:6379"))
	if len(redisAddrs) > 0 {
		if err := cache.InitRedis(redisAddrs, config.GetEnv("REDIS_PASSWORD", "")); err != nil {
			log.Printf("recommend-service redis disabled: %v", err)
		}
	}
}

func initRecommendMySQLWithFallback(host string, port int, dbName string) (*gorm.DB, error) {
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
