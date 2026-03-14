package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"xiangchisha/internal/distributed/contracts"
	"xiangchisha/internal/platform/cachex"
	"xiangchisha/internal/platform/config"
	"xiangchisha/pkg/cache"
	"xiangchisha/pkg/database"
)

type userRow struct {
	ID                 int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Username           string    `gorm:"column:username;size:50;uniqueIndex;not null"`
	Password           string    `gorm:"column:password;size:128"`
	PasswordHash       string    `gorm:"column:password_hash;size:128"`
	Phone              string    `gorm:"column:phone;size:20"`
	HasPreference      bool      `gorm:"column:has_preference;not null;default:false"`
	LocationPermission string    `gorm:"column:location_permission;size:20;not null;default:'unset'"`
	CreatedAt          time.Time `gorm:"column:created_at"`
}

func (userRow) TableName() string { return "users" }

type preferenceRow struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID        int64     `gorm:"column:user_id;uniqueIndex;not null"`
	HasPreference bool      `gorm:"column:has_preference;not null;default:false"`
	Categories    string    `gorm:"column:categories;type:text"`
	PriceRange    string    `gorm:"column:price_range;size:50"`
	Tastes        string    `gorm:"column:tastes;type:text"`
	Merchants     string    `gorm:"column:merchants;type:text"`
	DishKeywords  string    `gorm:"column:dish_keywords;type:text"`
	AvoidFoods    string    `gorm:"column:avoid_foods;type:text"`
	OrderTimes    string    `gorm:"column:order_times;type:text"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (preferenceRow) TableName() string { return "user_preferences" }

type UserService struct {
	mu       sync.RWMutex
	db       *gorm.DB
	users    map[string]userRow
	locPerms map[int64]string
	prefs    map[int64]contracts.UserPreference
	sessions map[string]int64
	nextID   int64
}

func NewUserService() *UserService {
	s := &UserService{
		users:    make(map[string]userRow),
		locPerms: make(map[int64]string),
		prefs:    make(map[int64]contracts.UserPreference),
		sessions: make(map[string]int64),
		nextID:   1000,
	}
	s.initStorage()
	return s
}

func (s *UserService) initStorage() {
	mysqlHost := config.GetEnv("MYSQL_HOST", "127.0.0.1")
	mysqlPort := config.GetEnvInt("MYSQL_PORT", 3306)
	mysqlDB := config.GetEnv("MYSQL_DB", "meituan_db_0")
	db, err := initMySQLWithFallback(mysqlHost, mysqlPort, mysqlDB)
	if err != nil {
		log.Fatalf("user-service mysql init failed: %v", err)
	} else {
		s.db = db
		if s.db != nil {
			_ = s.db.AutoMigrate(&userRow{}, &preferenceRow{})
		} else {
			log.Fatal("user-service mysql db instance is nil")
		}
	}
	redisAddrs := config.SplitCSV(config.GetEnv("REDIS_ADDRS", "127.0.0.1:6379"))
	if len(redisAddrs) > 0 {
		if err := cache.InitRedis(redisAddrs, config.GetEnv("REDIS_PASSWORD", "")); err != nil {
			log.Printf("user-service redis disabled: %v", err)
		}
	}
}

func (s *UserService) Register(_ context.Context, req *contracts.RegisterRequest) (*contracts.BaseResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" || len(req.Password) < 6 {
		return &contracts.BaseResponse{Code: 400, Message: "username/password invalid"}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[username]; ok {
		return &contracts.BaseResponse{Code: 409, Message: "username already exists"}, nil
	}
	if s.db != nil {
		var exists userRow
		if err := s.db.Where("username = ?", username).First(&exists).Error; err == nil {
			return &contracts.BaseResponse{Code: 409, Message: "username already exists"}, nil
		}
	}

	row := userRow{
		Username:           username,
		Password:           hashPassword(req.Password),
		PasswordHash:       hashPassword(req.Password),
		Phone:              strings.TrimSpace(req.Phone),
		HasPreference:      false,
		LocationPermission: "unset",
		CreatedAt:          time.Now(),
	}
	if s.db != nil {
		if err := s.persistUserRow(row); err != nil {
			log.Printf("user-service register persist failed: %v", err)
			return &contracts.BaseResponse{Code: 500, Message: "register persist failed"}, nil
		}
		var persisted userRow
		if err := s.db.Where("username = ?", username).First(&persisted).Error; err == nil && persisted.ID > 0 {
			row.ID = persisted.ID
		}
	} else {
		s.nextID++
		row.ID = s.nextID
	}
	s.users[username] = row
	return &contracts.BaseResponse{Code: 0, Message: "ok"}, nil
}

func (s *UserService) Login(_ context.Context, req *contracts.LoginRequest) (*contracts.LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	passwordHash := hashPassword(req.Password)

	s.mu.RLock()
	row, ok := s.users[username]
	s.mu.RUnlock()
	if !ok && s.db != nil {
		var dbRow userRow
		if err := s.db.Where("username = ?", username).First(&dbRow).Error; err == nil {
			row = dbRow
			ok = true
			s.mu.Lock()
			s.users[username] = dbRow
			s.mu.Unlock()
		}
	}
	if !ok || row.getPasswordHash() != passwordHash {
		return &contracts.LoginResponse{Code: 401, Message: "invalid username or password"}, nil
	}

	token := makeToken(row.ID, row.Username)
	s.mu.Lock()
	s.sessions[token] = row.ID
	s.mu.Unlock()
	return &contracts.LoginResponse{Code: 0, Message: "ok", Token: token, UserID: row.ID}, nil
}

func (s *UserService) ValidateToken(_ context.Context, req *contracts.ValidateTokenRequest) (*contracts.ValidateTokenResponse, error) {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return &contracts.ValidateTokenResponse{Code: 401, Message: "missing token", Valid: false}, nil
	}
	s.mu.RLock()
	userID, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return &contracts.ValidateTokenResponse{Code: 401, Message: "invalid token", Valid: false}, nil
	}
	return &contracts.ValidateTokenResponse{Code: 0, Message: "ok", Valid: true, UserID: userID}, nil
}

func (s *UserService) GetUserInfo(_ context.Context, req *contracts.UserIDRequest) (*contracts.MeResponse, error) {
	s.mu.RLock()
	for _, row := range s.users {
		if row.ID == req.UserID {
			s.mu.RUnlock()
			return &contracts.MeResponse{Code: 0, Message: "ok", Data: contracts.MeResponseData{UserID: row.ID, Username: row.Username, LocationPermission: normalizeLocationPermission(row.LocationPermission)}}, nil
		}
	}
	s.mu.RUnlock()
	if s.db != nil {
		var row userRow
		if err := s.db.Where("id = ?", req.UserID).First(&row).Error; err == nil {
			s.mu.Lock()
			s.users[row.Username] = row
			s.mu.Unlock()
			return &contracts.MeResponse{Code: 0, Message: "ok", Data: contracts.MeResponseData{UserID: row.ID, Username: row.Username, LocationPermission: normalizeLocationPermission(row.LocationPermission)}}, nil
		}
	}
	return &contracts.MeResponse{Code: 404, Message: "user not found"}, nil
}

func (s *UserService) GetPreference(ctx context.Context, req *contracts.UserIDRequest) (*contracts.PreferenceResponse, error) {
	cacheKey := fmt.Sprintf("user:pref:%d", req.UserID)
	hasPref, _ := s.getUserHasPreference(ctx, req.UserID)
	if !hasPref {
		return &contracts.PreferenceResponse{
			Code:    0,
			Message: "ok",
			Data: contracts.PreferenceResponseData{
				HasPreference: false,
				Preference:    nil,
			},
		}, nil
	}

	var pref contracts.UserPreference
	if err := cache.Get(ctx, cacheKey, &pref); err == nil {
		return &contracts.PreferenceResponse{
			Code:    0,
			Message: "ok",
			Data: contracts.PreferenceResponseData{
				HasPreference: true,
				Preference:    &pref,
			},
		}, nil
	}

	if s.db != nil {
		var row preferenceRow
		if err := s.db.Where("user_id = ?", req.UserID).First(&row).Error; err == nil {
			pref = decodePref(req.UserID, row)
			_ = cachex.SetWithJitter(ctx, cacheKey, pref, time.Hour, 300)
			return &contracts.PreferenceResponse{
				Code:    0,
				Message: "ok",
				Data: contracts.PreferenceResponseData{
					HasPreference: true,
					Preference:    &pref,
				},
			}, nil
		}
	}
	return &contracts.PreferenceResponse{
		Code:    0,
		Message: "ok",
		Data: contracts.PreferenceResponseData{
			HasPreference: false,
			Preference:    nil,
		},
	}, nil
}

func (s *UserService) UpdatePreference(ctx context.Context, req *contracts.UpdatePreferenceRequest) (*contracts.BaseResponse, error) {
	pref := contracts.UserPreference{
		UserID:       req.UserID,
		Categories:   req.Categories,
		PriceRange:   req.PriceRange,
		Tastes:       req.Tastes,
		Merchants:    req.Merchants,
		DishKeywords: req.DishKeywords,
		AvoidFoods:   req.AvoidFoods,
		OrderTimes:   req.OrderTimes,
	}

	if s.db != nil {
		row := encodePref(pref)
		row.UserID = req.UserID
		row.UpdatedAt = time.Now()
		if err := s.db.Where("user_id = ?", req.UserID).Assign(&row).FirstOrCreate(&preferenceRow{}).Error; err != nil {
			return &contracts.BaseResponse{Code: 500, Message: "update preference failed"}, nil
		}
	}
	cacheKey := fmt.Sprintf("user:pref:%d", req.UserID)
	_ = cache.Del(ctx, cacheKey)
	return &contracts.BaseResponse{Code: 0, Message: "ok"}, nil
}

func (s *UserService) GetLocationPermission(ctx context.Context, req *contracts.UserIDRequest) (*contracts.LocationPermissionResponse, error) {
	perm := "unset"
	cacheKey := locationPermissionCacheKey(req.UserID)
	s.mu.RLock()
	if cachedPerm, ok := s.locPerms[req.UserID]; ok {
		s.mu.RUnlock()
		return &contracts.LocationPermissionResponse{Code: 0, Message: "ok", LocationPermission: normalizeLocationPermission(cachedPerm)}, nil
	}
	s.mu.RUnlock()
	var redisPerm string
	if err := cache.Get(ctx, cacheKey, &redisPerm); err == nil {
		perm = normalizeLocationPermission(redisPerm)
		s.mu.Lock()
		s.locPerms[req.UserID] = perm
		s.mu.Unlock()
		return &contracts.LocationPermissionResponse{Code: 0, Message: "ok", LocationPermission: perm}, nil
	}
	s.mu.RLock()
	for _, row := range s.users {
		if row.ID == req.UserID {
			perm = normalizeLocationPermission(row.LocationPermission)
			s.mu.RUnlock()
			s.mu.Lock()
			s.locPerms[req.UserID] = perm
			s.mu.Unlock()
			_ = cachex.SetWithJitter(ctx, cacheKey, perm, 12*time.Hour, 600)
			return &contracts.LocationPermissionResponse{Code: 0, Message: "ok", LocationPermission: perm}, nil
		}
	}
	s.mu.RUnlock()
	if s.db != nil {
		var row userRow
		if err := s.db.Where("id = ?", req.UserID).First(&row).Error; err == nil {
			perm = normalizeLocationPermission(row.LocationPermission)
			s.mu.Lock()
			s.users[row.Username] = row
			s.locPerms[req.UserID] = perm
			s.mu.Unlock()
			_ = cachex.SetWithJitter(ctx, cacheKey, perm, 12*time.Hour, 600)
		}
	}
	return &contracts.LocationPermissionResponse{Code: 0, Message: "ok", LocationPermission: perm}, nil
}

func (s *UserService) UpdateLocationPermission(ctx context.Context, req *contracts.UpdateLocationPermissionRequest) (*contracts.BaseResponse, error) {
	perm := normalizeLocationPermission(req.LocationPermission)
	if perm != "always" && perm != "denied" && perm != "unset" {
		return &contracts.BaseResponse{Code: 400, Message: "invalid location permission"}, nil
	}
	cacheKey := locationPermissionCacheKey(req.UserID)
	s.mu.Lock()
	s.locPerms[req.UserID] = perm
	for username, row := range s.users {
		if row.ID != req.UserID {
			continue
		}
		row.LocationPermission = perm
		s.users[username] = row
		break
	}
	s.mu.Unlock()
	if s.db != nil {
		_ = s.db.Model(&userRow{}).Where("id = ?", req.UserID).Update("location_permission", perm).Error
	}
	_ = cachex.SetWithJitter(ctx, cacheKey, perm, 12*time.Hour, 600)
	return &contracts.BaseResponse{Code: 0, Message: "ok"}, nil
}

func locationPermissionCacheKey(userID int64) string {
	return fmt.Sprintf("user:location_permission:%d", userID)
}

func (s *UserService) getUserHasPreference(_ context.Context, userID int64) (bool, error) {
	if s.db == nil {
		return false, nil
	}
	var dbRow preferenceRow
	if err := s.db.Where("user_id = ?", userID).First(&dbRow).Error; err != nil {
		return false, nil
	}
	if dbRow.UserID <= 0 {
		return false, nil
	}
	if dbRow.HasPreference {
		return true, nil
	}
	return true, nil // 兼容历史数据：旧记录没有 has_preference 时按已填写处理
}

func hashPassword(raw string) string {
	sum := sha256.Sum256([]byte(raw + ":salt"))
	return hex.EncodeToString(sum[:])
}

func makeToken(userID int64, username string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%d", userID, username, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:])
}

func encodePref(pref contracts.UserPreference) preferenceRow {
	return preferenceRow{
		HasPreference: true,
		Categories:    toJSON(pref.Categories),
		PriceRange:    pref.PriceRange,
		Tastes:        toJSON(pref.Tastes),
		Merchants:     toJSON(pref.Merchants),
		DishKeywords:  toJSON(pref.DishKeywords),
		AvoidFoods:    toJSON(pref.AvoidFoods),
		OrderTimes:    toJSON(pref.OrderTimes),
	}
}

func decodePref(userID int64, row preferenceRow) contracts.UserPreference {
	var pref contracts.UserPreference
	pref.UserID = userID
	_ = fromJSON(row.Categories, &pref.Categories)
	pref.PriceRange = row.PriceRange
	_ = fromJSON(row.Tastes, &pref.Tastes)
	_ = fromJSON(row.Merchants, &pref.Merchants)
	_ = fromJSON(row.DishKeywords, &pref.DishKeywords)
	_ = fromJSON(row.AvoidFoods, &pref.AvoidFoods)
	_ = fromJSON(row.OrderTimes, &pref.OrderTimes)
	return pref
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func fromJSON(s string, out interface{}) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), out)
}

func normalizeLocationPermission(v string) string {
	val := strings.ToLower(strings.TrimSpace(v))
	if val == "always" || val == "denied" {
		return val
	}
	return "unset"
}

func (r userRow) getPasswordHash() string {
	if strings.TrimSpace(r.Password) != "" {
		return r.Password
	}
	return r.PasswordHash
}

func (s *UserService) persistUserRow(row userRow) error {
	base := map[string]interface{}{
		"username":            row.Username,
		"phone":               row.Phone,
		"has_preference":      row.HasPreference,
		"location_permission": row.LocationPermission,
		"created_at":          row.CreatedAt,
	}
	variants := []map[string]interface{}{
		mergeInsertMap(base, map[string]interface{}{"password": row.Password, "password_hash": row.PasswordHash}),
		mergeInsertMap(base, map[string]interface{}{"password": row.Password}),
		mergeInsertMap(base, map[string]interface{}{"password_hash": row.PasswordHash}),
	}
	var lastErr error
	for _, data := range variants {
		if err := s.db.Table("users").Create(data).Error; err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func mergeInsertMap(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func initMySQLWithFallback(host string, port int, dbName string) (*gorm.DB, error) {
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
