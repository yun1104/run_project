package cachex

import (
	"context"
	"math/rand"
	"time"

	"xiangchisha/pkg/cache"
)

func SetWithJitter(ctx context.Context, key string, value interface{}, baseTTL time.Duration, jitterSeconds int) error {
	if jitterSeconds <= 0 {
		return cache.Set(ctx, key, value, baseTTL)
	}
	ttl := baseTTL + time.Duration(rand.Intn(jitterSeconds))*time.Second
	return cache.Set(ctx, key, value, ttl)
}
