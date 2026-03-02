# 技术学习路线

> 基于本项目技术栈，按优先级排序。每个阶段完成后再进入下一阶段。

---

## 阶段一：Go 语言基础（2周）

### 必学
- [ ] goroutine 和 channel 基本用法
- [ ] sync 包：Mutex、RWMutex、WaitGroup、Once
- [ ] atomic 包：AddInt64、LoadInt64、CompareAndSwap
- [ ] context：WithTimeout、WithCancel、WithValue
- [ ] defer/panic/recover
- [ ] interface 和组合模式
- [ ] error 处理惯用法

### 验证
能独立写出：
```go
// 并发安全的计数器
type Counter struct {
    val int64
}
func (c *Counter) Inc() { atomic.AddInt64(&c.val, 1) }
func (c *Counter) Get() int64 { return atomic.LoadInt64(&c.val) }
```

### 资料
- 《Go程序设计语言》第1-8章
- https://go.dev/tour/

---

## 阶段二：Go 并发进阶（1周）

### 必学
- [ ] GMP 调度模型（G=goroutine, M=线程, P=处理器）
- [ ] goroutine 池（工作池模式）
- [ ] select + channel 超时控制
- [ ] sync.Map 适用场景
- [ ] `go run -race` 数据竞争检测
- [ ] 内存逃逸分析

### 重点理解
```
为什么 goroutine 比线程轻量？
- 初始栈 2KB（线程默认 8MB）
- GMP 用户态调度，不陷入内核
- 阻塞时 P 可以切换其他 G
```

### 验证
能手写 WorkerPool：
- 固定 N 个 worker goroutine
- 任务队列 buffered channel
- 优雅关闭（Stop + 等待 WaitGroup）

---

## 阶段三：Gin 框架（3天）

### 必学
- [ ] 路由分组、参数绑定（ShouldBindJSON）
- [ ] 中间件链（Use、Next、Abort）
- [ ] 自定义中间件：日志、鉴权、限流
- [ ] 错误统一处理

### 对应代码
`api-gateway/main.go` - 网关路由注册和中间件

---

## 阶段四：MySQL + GORM（1周）

### 必学
- [ ] 索引原理：B+树、联合索引最左前缀
- [ ] 事务隔离级别：RU/RC/RR/Serializable
- [ ] MVCC 实现原理（undo log + read view）
- [ ] 锁：行锁、间隙锁、意向锁
- [ ] EXPLAIN 分析执行计划
- [ ] 慢查询：避免 SELECT *，分页用覆盖索引

### 分库分表（重点）
```
本项目方案：
- user_preference：按 user_id % 10 → 库，user_id % 100 → 表
- orders：按月 range 分片，orders_202601

面试必答：
Q: 跨分片查询怎么做？
A: 业务层聚合 or 引入 ShardingSphere
Q: 分表后 id 如何保证全局唯一？
A: 雪花算法 or 数据库号段模式
```

### GORM
- [ ] AutoMigrate、Model 定义
- [ ] 事务 Transaction
- [ ] 软删除 DeletedAt

---

## 阶段五：Redis（1周）

### 必学
- [ ] 5种数据结构及使用场景（String/Hash/List/Set/ZSet）
- [ ] 持久化：RDB vs AOF
- [ ] 过期策略：惰性删除 + 定期删除
- [ ] 内存淘汰策略：LRU/LFU/allkeys

### 本项目用到的
```
String：用户偏好 JSON 缓存（TTL 1h）
ZSet：热门商家排行榜（score=订单量）
SetNX：分布式锁
```

### 缓存三件套（必须手写思路）
| 问题 | 原因 | 解决方案 |
|---|---|---|
| 穿透 | 查询不存在的 key | 布隆过滤器 |
| 击穿 | 热点 key 过期瞬间 | 互斥锁 or 永不过期 |
| 雪崩 | 大量 key 同时过期 | 随机 TTL + 熔断 |

### 分布式锁（面试高频，必须手写）
```go
// 加锁
SET lock:key uuid NX EX 10

// 解锁（Lua 保证原子性）
if redis.call("get",KEYS[1])==ARGV[1] then
    return redis.call("del",KEYS[1])
end
```
关键点：value 用 UUID 防误删、TTL 防死锁、Lua 原子解锁

---

## 阶段六：消息队列 Kafka（4天）

### 必学
- [ ] 核心概念：Producer、Consumer、Topic、Partition、Offset、ConsumerGroup
- [ ] 为什么快：顺序写磁盘 + 零拷贝 sendfile
- [ ] 消息可靠性：acks=all + ISR
- [ ] 重复消费处理：幂等性设计（唯一 ID 去重）

### 本项目用到的 Topic
```
order.history  → 爬虫投递历史订单 → preference-service 消费分析偏好
order.create   → 用户下单 → order-service 处理
order.paid     → 支付完成 → notification-service 通知
```

### 面试答法
```
Q: 如何保证消息不丢失？
A: Producer acks=all + Consumer 手动提交 offset + 磁盘持久化

Q: 如何处理重复消费？
A: 消费者业务幂等，用唯一 ID 在 Redis/DB 去重
```

---

## 阶段七：gRPC + Protobuf（3天）

### 必学
- [ ] .proto 文件语法：message、service、rpc
- [ ] 生成 Go 代码：protoc 命令
- [ ] 四种 RPC 模式：Unary / Server Stream / Client Stream / Bidirectional
- [ ] gRPC 拦截器（相当于中间件）

### 为什么比 REST 快
```
JSON：文本序列化，可读性好，体积大
Protobuf：二进制序列化，体积小 3-5x，解析快 10x
HTTP/2：多路复用，一个 TCP 连接处理多个请求
```

### 对应代码
`proto/user.proto`、`proto/order.proto`、`proto/recommend.proto`

---

## 阶段八：微服务治理（1周）

### 服务注册发现 Consul
- [ ] 服务注册：启动时上报 IP:Port
- [ ] 健康检查：HTTP /health 心跳
- [ ] 服务发现：客户端查询可用实例列表

### 限流（本项目：令牌桶）
```
令牌桶：以固定速率放 token，请求消耗 token，允许突发
漏桶：匀速处理，不允许突发
滑动窗口：Redis 计数，适合分布式场景
```

### 熔断（Sentinel-Go）
```
状态机：Closed → Open → Half-Open
连续失败 5 次 → 熔断 30s → 放一个探测请求 → 成功则恢复
降级：AI 超时 300ms → 返回热门推荐（规则兜底）
```

### 链路追踪 Jaeger
- [ ] Trace / Span 概念
- [ ] 如何注入到 gRPC/HTTP Header
- 本项目案例：Jaeger 发现 AI 调用耗时 2.5s → 加缓存+超时降级 → RT 降至 200ms

---

## 阶段九：分布式事务（3天）

### Saga 模式（本项目使用）
```
下单流程：
1. 锁库存      失败 → 直接返回
2. 创建订单    失败 → 解锁库存
3. 调用支付    失败 → 取消订单 → 解锁库存
4. 确认订单    失败 → 退款 → 取消订单 → 解锁库存

特点：最终一致性，无全局锁，适合长流程
```

### 对比其他方案
| 方案 | 一致性 | 性能 | 适用场景 |
|---|---|---|---|
| 2PC | 强一致 | 差（阻塞） | 短事务 |
| Saga | 最终一致 | 好 | 长业务流程 |
| TCC | 最终一致 | 中 | 对一致性要求高 |
| 消息事务 | 最终一致 | 好 | 跨服务异步 |

---

## 阶段十：监控体系（2天）

### Prometheus + Grafana
- [ ] 四种指标类型：Counter/Gauge/Histogram/Summary
- [ ] PromQL 基础查询
- 本项目监控：QPS、RT（P99）、错误率、缓存命中率

### 告警阈值（面试可说）
```
错误率 > 5%  → 告警
P99 RT > 1s  → 告警
QPS 异常波动 → 告警
```

---

## 面试速查表

### 数据指标（背下来）
```
QPS：单机 5000+，集群可扩 100w+
RT：P99 < 200ms
可用性：99.9%
缓存命中率：95%+
数据规模：亿级订单
```

### 性能优化案例（必须能讲）
```
问题：推荐服务 RT 高达 3s
定位：Jaeger 发现 AI 调用耗时 2.5s，召回无超时
解决：
  1. AI 调用超时控制 300ms
  2. 召回结果缓存 10min
  3. 本地缓存热门商家
  4. AI 超时降级规则推荐
结果：RT 降至 200ms，提升 15x
```

### 诚实口径（重要）
```
目前主链路（网关+MySQL+Redis）已完整运行。
微服务拆分、Kafka 流、分布式事务的代码模块已设计完成，
基础设施编排在 docker-compose.yml 中已定义，
下一步是完成服务间 gRPC 通信的接入。
```
