package cache

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"time"
)

var rdb redis.UniversalClient

func InitRedis(addrs []string, password string) error {
	rdb = redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        addrs,
		Password:     password,
		PoolSize:     100,
		MinIdleConns: 20,
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	return rdb.Ping(ctx).Err()
}

func Get(ctx context.Context, key string, dest interface{}) error {
	if rdb == nil {
		return redis.Nil
	}
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if rdb == nil {
		return redis.Nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, key, data, expiration).Err()
}

func Del(ctx context.Context, keys ...string) error {
	if rdb == nil {
		return redis.Nil
	}
	return rdb.Del(ctx, keys...).Err()
}

func GetClient() redis.UniversalClient {
	return rdb
}
