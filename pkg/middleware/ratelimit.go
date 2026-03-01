package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	requests map[string]*RequestInfo
	mu       sync.Mutex
	rate     int
	window   time.Duration
}

type RequestInfo struct {
	count     int
	resetTime time.Time
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string]*RequestInfo),
		rate:     rate,
		window:   window,
	}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		
		rl.mu.Lock()
		defer rl.mu.Unlock()
		
		now := time.Now()
		info, exists := rl.requests[ip]
		
		if !exists || now.After(info.resetTime) {
			rl.requests[ip] = &RequestInfo{
				count:     1,
				resetTime: now.Add(rl.window),
			}
			c.Next()
			return
		}
		
		if info.count >= rl.rate {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "Too many requests",
			})
			c.Abort()
			return
		}
		
		info.count++
		c.Next()
	}
}
