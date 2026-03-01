package lock

import (
	"context"
	"errors"
	"github.com/go-redis/redis/v8"
	"time"
)

type DistributedLock struct {
	client *redis.ClusterClient
	key    string
	value  string
	ttl    time.Duration
}

func NewDistributedLock(client *redis.ClusterClient, key, value string, ttl time.Duration) *DistributedLock {
	return &DistributedLock{
		client: client,
		key:    key,
		value:  value,
		ttl:    ttl,
	}
}

func (l *DistributedLock) Lock(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (l *DistributedLock) Unlock(ctx context.Context) error {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	_, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	return err
}

func (l *DistributedLock) TryLockWithRetry(ctx context.Context, maxRetry int, retryInterval time.Duration) error {
	for i := 0; i < maxRetry; i++ {
		ok, err := l.Lock(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		time.Sleep(retryInterval)
	}
	return errors.New("failed to acquire lock after retries")
}
