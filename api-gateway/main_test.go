package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHashPassword(t *testing.T) {
	got := hashPassword("123456")
	if got == "" {
		t.Fatal("hash should not be empty")
	}
	if got == "123456" {
		t.Fatal("hash should not equal plain password")
	}
	if got != hashPassword("123456") {
		t.Fatal("same password should produce the same hash")
	}
	if got == hashPassword("abcdef") {
		t.Fatal("different passwords should produce different hashes")
	}
}

func TestRecommendByRequirement(t *testing.T) {
	tests := []struct {
		name        string
		requirement string
		wantFirstID int64
		wantCategory string
	}{
		{name: "empty requirement returns default merchants", requirement: "", wantFirstID: 10001, wantCategory: "快餐"},
		{name: "light food requirement returns light merchants", requirement: "想吃轻食，控制热量", wantFirstID: 20001, wantCategory: "轻食"},
		{name: "fat loss requirement returns light merchants", requirement: "减脂高蛋白", wantFirstID: 20001, wantCategory: "轻食"},
		{name: "spicy requirement returns Sichuan merchants", requirement: "今天想吃辣", wantFirstID: 30001, wantCategory: "川菜"},
		{name: "Sichuan requirement returns Sichuan merchants", requirement: "川菜午餐", wantFirstID: 30001, wantCategory: "川菜"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merchants := recommendByRequirement(tt.requirement)
			if len(merchants) == 0 {
				t.Fatal("expected merchants, got empty list")
			}
			if merchants[0].ID != tt.wantFirstID {
				t.Fatalf("first merchant id = %d, want %d", merchants[0].ID, tt.wantFirstID)
			}
			if merchants[0].Category != tt.wantCategory {
				t.Fatalf("first merchant category = %q, want %q", merchants[0].Category, tt.wantCategory)
			}
		})
	}
}

func TestPreferenceRowConversion(t *testing.T) {
	pref := UserPreference{
		UserID:       1001,
		SpicyLevel:   "中辣",
		BudgetRange:  "20-40元",
		CuisineLikes: []string{"川菜", "快餐"},
		AvoidFoods:   []string{"花生"},
		DietGoal:     "随意吃",
		DiningTime:   "午餐",
		UpdatedAt:    time.Now().Format(time.RFC3339),
	}

	row := preferenceToRow(pref)
	got := rowToPreference(row)

	if got.UserID != pref.UserID ||
		got.SpicyLevel != pref.SpicyLevel ||
		got.BudgetRange != pref.BudgetRange ||
		got.DietGoal != pref.DietGoal ||
		got.DiningTime != pref.DiningTime {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, pref)
	}
	if len(got.CuisineLikes) != 2 || got.CuisineLikes[0] != "川菜" || got.CuisineLikes[1] != "快餐" {
		t.Fatalf("cuisine likes mismatch: %+v", got.CuisineLikes)
	}
	if len(got.AvoidFoods) != 1 || got.AvoidFoods[0] != "花生" {
		t.Fatalf("avoid foods mismatch: %+v", got.AvoidFoods)
	}
}

func TestMustAuthUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisReady = false
	storeMu.Lock()
	sessions = map[string]int64{"valid-token": 42}
	storeMu.Unlock()

	t.Run("missing token", func(t *testing.T) {
		ctx, recorder := testContext(http.MethodGet, "/api/v1/user/me")
		if userID, ok := mustAuthUserID(ctx); ok || userID != 0 {
			t.Fatalf("expected auth failure, got userID=%d ok=%v", userID, ok)
		}
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		ctx, recorder := testContext(http.MethodGet, "/api/v1/user/me")
		ctx.Request.Header.Set("Authorization", "Bearer bad-token")
		if userID, ok := mustAuthUserID(ctx); ok || userID != 0 {
			t.Fatalf("expected auth failure, got userID=%d ok=%v", userID, ok)
		}
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		ctx, recorder := testContext(http.MethodGet, "/api/v1/user/me")
		ctx.Request.Header.Set("Authorization", "Bearer valid-token")
		userID, ok := mustAuthUserID(ctx)
		if !ok || userID != 42 {
			t.Fatalf("userID=%d ok=%v, want userID=42 ok=true", userID, ok)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
	})
}

func TestHostLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(HostLimitMiddleware("allowed.example.com"))
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	t.Run("allowed configured host", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Host = "allowed.example.com"
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
	})

	t.Run("localhost remains allowed", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Host = "localhost:8080"
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
	})

	t.Run("unknown host is rejected", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Host = "evil.example.com"
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
	})
}

func testContext(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, nil)
	return ctx, recorder
}
