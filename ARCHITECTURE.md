# AI外卖推荐Agent系统架构设计

## 技术栈

- **后端框架**: Go 1.21+ + Gin
- **数据库**: MySQL 8.0(分库分表) + Redis Cluster
- **消息队列**: Kafka
- **服务治理**: Consul + gRPC
- **网关**: Gin Gateway
- **爬虫**: Playwright
- **AI模型**: 通义千问/文心一言 API
- **监控**: Prometheus + Grafana + Jaeger
- **限流熔断**: Sentinel-Go

## 微服务拆分

```
├── api-gateway           // API网关
├── user-service          // 用户服务
├── order-service         // 订单服务
├── preference-service    // 偏好分析服务
├── recommend-service     // 推荐服务
├── meituan-crawler       // 美团爬虫服务
├── payment-service       // 支付服务
├── ai-service            // AI服务
└── notification-service  // 通知服务
```

## 核心功能

### 1. 用户偏好管理
- MySQL分表存储：user_preference_0~99（按user_id取模）
- Redis缓存热点用户偏好
- 支持偏好CRUD操作

### 2. 订单分析与偏好提取
- 爬虫获取历史订单 → Kafka → 入库
- AI批量分析订单提取偏好
- 增量更新用户偏好文件

### 3. 智能推荐
- 多路召回：偏好召回、协同过滤、热门商家
- AI排序打分
- 三级缓存优化

### 4. 自动下单
- 爬虫池负载均衡
- 反反爬虫策略：代理池、验证码识别
- Saga分布式事务

## 高并发方案

- API网关限流：令牌桶算法
- 三级缓存：本地缓存 + Redis + MySQL
- 数据库分库分表：10库100表
- 服务降级熔断：Sentinel-Go
- goroutine并发处理：工作池模式

## 并发安全保障

- 分布式锁：Redis SetNX防重复下单
- 乐观锁：version字段防库存超卖
- 原子操作：atomic包处理计数器
- 读写锁：sync.RWMutex优化缓存
- 事务隔离：REPEATABLE READ + FOR UPDATE

## 项目亮点

1. 分布式微服务架构
2. 分库分表支持亿级数据
3. 三级缓存设计
4. Kafka消息驱动
5. AI偏好分析与推荐
6. 爬虫反反爬策略
7. 服务治理完整方案
