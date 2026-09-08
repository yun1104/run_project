package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterAllowsRequestsWithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(2, time.Minute)
	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d", i+1, recorder.Code, http.StatusOK)
		}
	}
}

func TestRateLimiterRejectsRequestsOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(1, time.Minute)
	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", first.Code, http.StatusOK)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
}
