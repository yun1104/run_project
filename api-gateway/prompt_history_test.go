package main

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
	"xiangchisha/pkg/database"
)

const testUserID = 888888

func TestPromptHistorySentToLLM(t *testing.T) {
	if err := initTestDB(); err != nil {
		t.Skipf("skip: mysql not available: %v", err)
	}
	ctx := context.Background()
	db := database.GetDB(testUserID)
	if db == nil {
		t.Fatal("db nil")
	}

	now := time.Now()
	orderRow := OrderHistoryRow{
		UserID:       testUserID,
		MerchantID:   10001,
		MerchantName: "鲜香牛肉饭",
		Amount:       29.5,
		Status:       "paid",
		Paid:         true,
		CreatedAt:    now,
		PaidAt:       now,
	}
	if err := db.Create(&orderRow).Error; err != nil {
		t.Fatalf("insert order_history: %v", err)
	}

	reqRow := RequirementHistoryRow{
		UserID:      testUserID,
		Requirement: "想吃辣的，预算40以内",
		CreatedAt:   now,
	}
	if err := db.Create(&reqRow).Error; err != nil {
		t.Fatalf("insert requirement_history: %v", err)
	}

	prefRow := UserPreferenceRow{
		UserID:      testUserID,
		SpicyLevel:  "中辣",
		BudgetRange: "20-40元",
		UpdatedAt:   now,
	}
	if err := db.Create(&prefRow).Error; err != nil {
		t.Fatalf("insert user_preferences: %v", err)
	}

	orderHistory, reqHistory := loadPromptHistory(ctx, testUserID)
	if len(orderHistory) == 0 {
		t.Error("order_history empty, expected non-empty")
	}
	if len(reqHistory) == 0 {
		t.Error("requirement_history empty, expected non-empty")
	}

	pref, hasPref := fetchPreference(ctx, testUserID)
	if !hasPref {
		t.Error("expected has_pref true")
	}

	input := llmPythonInput{
		Requirement:        "再来一份",
		HasPref:            hasPref,
		Preference:         pref,
		Candidates:         []Merchant{{ID: 10001, Name: "鲜香牛肉饭"}},
		OrderHistory:       orderHistory,
		RequirementHistory: reqHistory,
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	jsonStr := string(inputBytes)

	if !strings.Contains(jsonStr, "鲜香牛肉饭") {
		t.Error("expected order_history to contain 鲜香牛肉饭")
	}
	if !strings.Contains(jsonStr, "想吃辣的，预算40以内") {
		t.Error("expected requirement_history to contain 想吃辣的，预算40以内")
	}
	if !strings.Contains(jsonStr, "中辣") || !strings.Contains(jsonStr, "20-40元") {
		t.Error("expected preference to contain 中辣 and 20-40元")
	}

	var parsed struct {
		OrderHistory       []OrderHistoryPrompt     `json:"order_history"`
		RequirementHistory []RequirementHistoryItem `json:"requirement_history"`
		Preference         UserPreference          `json:"preference"`
	}
	if err := json.Unmarshal(inputBytes, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.OrderHistory) == 0 {
		t.Error("parsed order_history empty")
	}
	if len(parsed.RequirementHistory) == 0 {
		t.Error("parsed requirement_history empty")
	}
	if parsed.Preference.SpicyLevel != "中辣" || parsed.Preference.BudgetRange != "20-40元" {
		t.Errorf("preference mismatch: got spicy=%s budget=%s", parsed.Preference.SpicyLevel, parsed.Preference.BudgetRange)
	}
}

func initTestDB() error {
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
		return nil
	}
	return db.AutoMigrate(&UserAccount{}, &UserPreferenceRow{}, &OrderHistoryRow{}, &RequirementHistoryRow{})
}
