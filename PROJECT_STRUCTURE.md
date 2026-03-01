# 项目结构说明

```
meituan-ai-agent/
├── api-gateway/                    # API网关
│   └── main.go                     # 网关入口，路由转发、限流
│
├── services/                       # 微服务
│   ├── user-service/               # 用户服务
│   │   ├── models/user.go          # 用户模型、偏好模型
│   │   └── service/user_service.go # 用户业务逻辑、偏好CRUD
│   │
│   ├── order-service/              # 订单服务
│   │   └── models/order.go         # 订单模型、分表逻辑
│   │
│   ├── preference-service/         # 偏好分析服务
│   │   └── service/preference.go   # AI偏好提取、增量更新
│   │
│   ├── recommend-service/          # 推荐服务
│   │   └── service/recommend.go    # 多路召回、AI排序
│   │
│   ├── meituan-crawler/            # 美团爬虫服务
│   │   └── crawler/meituan.go      # Playwright自动化、浏览器池
│   │
│   ├── payment-service/            # 支付服务
│   │   └── service/payment.go      # Saga分布式事务
│   │
│   ├── ai-service/                 # AI服务
│   │   └── client/ai_client.go     # AI API封装、prompt工程
│   │
│   └── notification-service/       # 通知服务
│
├── pkg/                            # 公共包
│   ├── database/mysql.go           # MySQL分库连接池
│   ├── cache/redis.go              # Redis Cluster封装
│   ├── mq/kafka.go                 # Kafka生产者消费者
│   ├── metrics/prometheus.go       # Prometheus指标
│   ├── tracing/jaeger.go           # Jaeger链路追踪
│   └── middleware/                 # 中间件
│       ├── ratelimit.go            # 限流中间件
│       └── circuit_breaker.go      # 熔断器
│
├── proto/                          # gRPC协议
│   ├── user.proto                  # 用户服务协议
│   ├── order.proto                 # 订单服务协议
│   └── recommend.proto             # 推荐服务协议
│
├── configs/                        # 配置文件
│   └── config.yaml                 # 全局配置
│
├── scripts/                        # 脚本
│   ├── docker-compose.yml          # 基础设施编排
│   ├── prometheus.yml              # Prometheus配置
│   ├── init_db.sql                 # 数据库初始化
│   └── deploy.sh                   # 部署脚本
│
├── go.mod                          # Go依赖管理
├── ARCHITECTURE.md                 # 架构设计文档
├── DEPLOYMENT.md                   # 部署指南
├── INTERVIEW.md                    # 面试要点
└── README.md                       # 项目说明
```

## 核心模块

### 1. API网关（api-gateway/main.go）
- 统一入口，路由转发
- 限流中间件
- 请求日志记录

### 2. 用户服务（user-service）
- 用户注册登录
- 偏好CRUD操作
- Redis缓存用户偏好
- MySQL分表存储

### 3. 订单服务（order-service）
- 历史订单查询
- 订单创建
- MySQL按月分表
- Kafka生产订单消息

### 4. 偏好分析服务（preference-service）
- 消费Kafka订单消息
- AI分析提取偏好
- 增量更新用户偏好
- 批量处理优化

### 5. 推荐服务（recommend-service）
- 多路召回并发执行
- AI排序打分
- 推荐结果缓存
- 降级策略

### 6. 爬虫服务（meituan-crawler）
- Playwright浏览器池
- 反反爬虫策略
- 订单爬取
- 自动下单支付

### 7. 支付服务（payment-service）
- Saga分布式事务
- 补偿机制
- 支付状态通知

### 8. AI服务（ai-service）
- 封装AI API调用
- 超时控制
- 重试机制
- Prompt工程

## 基础设施

### MySQL
- 10个数据库实例
- 每库100张表（偏好表）
- 按月分表（订单表）
- 主从复制、读写分离

### Redis Cluster
- 3主3从集群
- 用户偏好缓存（TTL 1h）
- 推荐结果缓存（TTL 10min）
- 热门商家排行榜（sorted set）

### Kafka
- 3个broker
- 主题：
  - order.history：历史订单
  - order.create：创建订单
  - order.paid：支付成功

### Consul
- 服务注册与发现
- 健康检查

### 监控
- Prometheus：指标采集
- Grafana：可视化监控
- Jaeger：链路追踪

## 数据流

### 偏好分析流
```
爬虫获取历史订单 
  → Kafka(order.history) 
  → preference-service消费 
  → AI分析提取偏好 
  → 更新user-service偏好表 
  → 清除Redis缓存
```

### 推荐流
```
用户请求 
  → API网关 
  → recommend-service 
  → 查询用户偏好（Redis/MySQL） 
  → 多路召回（并发） 
  → AI排序 
  → 返回Top N
```

### 下单流
```
用户选择商家 
  → API网关 
  → payment-service 
  → Saga事务：
     1. 锁定库存
     2. 创建订单
     3. 调用爬虫下单
     4. 支付
     5. 确认
  → Kafka(order.paid) 
  → notification-service通知用户
```
