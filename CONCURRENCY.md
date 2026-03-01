# 并发安全设计

## 1. 分布式锁

### 场景：防止重复下单

用户快速点击下单按钮，可能导致重复订单。

**方案**：Redis分布式锁

```go
lockKey := fmt.Sprintf("order:lock:%d:%d", userID, merchantID)
distLock := lock.NewDistributedLock(client, lockKey, "order-create", 10*time.Second)

err := distLock.TryLockWithRetry(ctx, 3, 100*time.Millisecond)
if err != nil {
    return errors.New("too many concurrent orders")
}
defer distLock.Unlock(ctx)

// 创建订单逻辑
```

**要点**�?
- SetNX原子操作
- Lua脚本保证unlock原子性（防止误删其他进程的锁）
- 设置TTL防止死锁
- 重试机制

### 场景：库存扣减

高并发下多个订单同时扣减库存，可能超卖。

**方案1：乐观锁（版本号）****

```go
UPDATE inventory 
SET stock = stock - ?, version = version + 1 
WHERE id = ? AND version = ? AND stock >= ?
```

- 无锁开销，性能高
- 适合冲突少的场景
- 失败重试（最多3次）

**方案2：悲观锁（FOR UPDATE）****

```go
SELECT stock FROM inventory WHERE id = ? FOR UPDATE
UPDATE inventory SET stock = stock - ? WHERE id = ?
```

- 阻塞等待，性能较低
- 适合冲突多的场景
- 保证强一致性

**方案3：Redis原子操作**

```go
redis.DecrBy("inventory:123", 5)
```

- 性能最高
- 需要定期同步到MySQL
- 适合高并发场景

## 2. 并发控制

### goroutine池

防止goroutine无限制创建导致OOM。

```go
pool := concurrency.NewWorkerPool(100, 1000)

for _, order := range orders {
    pool.Submit(func() {
        processOrder(order)
    })
}

pool.Stop()
```

**参数**：
- workers: 100个工作协程
- queueSize: 1000任务队列
- 超出队列拒绝请求

### 信号量控制并发度

```go
semaphore := make(chan struct{}, 10)

for _, order := range orders {
    semaphore <- struct{}{}
    go func(o Order) {
        defer func() { <-semaphore }()
        processOrder(o)
    }(order)
}
```

限制同时运行的goroutine数量为10。

## 3. 数据竞争

### 原子操作

统计请求数量：

```go
type OrderService struct {
    requestCount int64  // 使用atomic操作
}

func (s *OrderService) Handle() {
    atomic.AddInt64(&s.requestCount, 1)
}

func (s *OrderService) GetCount() int64 {
    return atomic.LoadInt64(&s.requestCount)
}
```

**禁止**：直接 `s.requestCount++`（数据竞争）

### 读写锁

缓存场景：读多写少

```go
type Cache struct {
    mu   sync.RWMutex
    data map[string]interface{}
}

func (c *Cache) Get(key string) interface{} {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.data[key]
}

func (c *Cache) Set(key string, val interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = val
}
```

**要点**�?
- 多个goroutine可同时读
- 写时独占�?
- 性能优于Mutex

### sync.Map

高并发map操作�?

```go
var cache sync.Map

cache.Store("key", value)
val, ok := cache.Load("key")
cache.Delete("key")
```

适合key稳定、读多写少的场景�?

## 4. 事务并发

### MySQL事务隔离级别

默认：REPEATABLE READ

**问题场景**�?

```sql
-- Session 1
BEGIN;
SELECT stock FROM inventory WHERE id = 1;  -- 100

-- Session 2
BEGIN;
UPDATE inventory SET stock = 90 WHERE id = 1;
COMMIT;

-- Session 1
UPDATE inventory SET stock = 95 WHERE id = 1;  -- 基于旧�?100
COMMIT;  -- 丢失更新
```

**解决**�?
- 悲观锁：`SELECT ... FOR UPDATE`
- 乐观锁：version字段
- 串行化隔离级别（性能差）

### 分布式事�?

Saga模式�?

```go
saga := []Step{
    {Execute: lockStock, Compensate: unlockStock},
    {Execute: createOrder, Compensate: cancelOrder},
    {Execute: payment, Compensate: refund},
}

for i, step := range saga {
    if err := step.Execute(); err != nil {
        for j := i - 1; j >= 0; j-- {
            saga[j].Compensate()
        }
        return err
    }
}
```

保证最终一致性�?

## 5. 缓存并发

### 缓存击穿

热点key过期，大量请求打到DB�?

**方案1：互斥锁**

```go
func GetWithMutex(key string) (interface{}, error) {
    val, err := redis.Get(key)
    if err == nil {
        return val, nil
    }
    
    lockKey := "lock:" + key
    lock := NewDistributedLock(lockKey)
    
    if lock.TryLock() {
        defer lock.Unlock()
        
        val := queryDB(key)
        redis.Set(key, val, 1*time.Hour)
        return val, nil
    }
    
    time.Sleep(100 * time.Millisecond)
    return GetWithMutex(key)
}
```

**方案2：永不过�?**

后台异步更新缓存�?

### 缓存穿�?

查询不存在的数据，每次都打DB。

**方案**：布隆过滤器

```go
if !bloomFilter.Exists(key) {
    return nil, errors.New("not found")
}
```

### 缓存雪崩

大量key同时过期�?

**方案**�?
- TTL加随机值：`ttl + rand(60)`
- 热点数据永不过期
- 熔断降级

## 6. 限流

### 令牌桶算�?

```go
type RateLimiter struct {
    tokens    int
    maxTokens int
    refillRate int
    lastRefill time.Time
    mu        sync.Mutex
}

func (r *RateLimiter) Allow() bool {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    now := time.Now()
    elapsed := now.Sub(r.lastRefill)
    r.tokens += int(elapsed.Seconds()) * r.refillRate
    if r.tokens > r.maxTokens {
        r.tokens = r.maxTokens
    }
    r.lastRefill = now
    
    if r.tokens > 0 {
        r.tokens--
        return true
    }
    return false
}
```

### 滑动窗口

Redis实现�?

```go
now := time.Now().Unix()
key := fmt.Sprintf("ratelimit:%s:%d", userID, now/60)

count, _ := redis.Incr(key)
redis.Expire(key, 60)

if count > 100 {
    return errors.New("rate limit exceeded")
}
```

## 7. 死锁预防

### 锁顺�?

```go
// 错误：可能死�?
func transfer(from, to *Account, amount int) {
    from.mu.Lock()
    to.mu.Lock()
    // ...
    to.mu.Unlock()
    from.mu.Unlock()
}

// 正确：按ID排序加锁
func transfer(from, to *Account, amount int) {
    if from.ID < to.ID {
        from.mu.Lock()
        to.mu.Lock()
    } else {
        to.mu.Lock()
        from.mu.Lock()
    }
    // ...
}
```

### 超时机制

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

select {
case <-ctx.Done():
    return errors.New("timeout")
case result := <-ch:
    return result
}
```

## 面试要点

### 1. 如何保证订单不重复创建？

分布式锁 + 唯一索引 + 幂等性设计

### 2. 高并发下如何防止库存超卖�?

乐观锁（version�?+ Redis原子操作 + 异步对账

### 3. 遇到过数据竞争问题吗？怎么解决的？

使用`go run -race`检测，用atomic/mutex/channel解决

### 4. 如何设计一个线程安全的LRU缓存�?

sync.RWMutex + 双向链表 + map

### 5. 分布式锁的实现原理？

Redis SetNX + Lua脚本unlock + TTL防死�? + 续期机制

### 6. 如何避免死锁�?

锁排�? + 超时机制 + 减少锁粒�? + 使用tryLock
