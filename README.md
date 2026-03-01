# AI外卖推荐Agent系统

基于Go语言开发的百万级高并发AI外卖推荐微服务系统。

## 架构特点

- 微服务架构：8+服务，gRPC通信
- 分库分表：10库100表，支持亿级数据
- 三级缓存：本地缓存 + Redis Cluster + MySQL
- 消息驱动：Kafka异步解耦
- AI应用：大模型偏好分析与推荐
- 爬虫技术：反反爬虫策略
- 服务治理：限流熔断降级

## 项目结构

```
├── api-gateway          # API网关
├── services/            # 微服务
│   ├── user-service
│   ├── order-service
│   ├── preference-service
│   ├── recommend-service
│   ├── meituan-crawler
│   ├── payment-service
│   ├── ai-service
│   └── notification-service
├── pkg/                 # 公共包
│   ├── database/
│   ├── cache/
│   └── mq/
├── proto/              # gRPC协议
├── configs/            # 配置文件
└── scripts/            # 脚本
```

## 快速开始

1. 初始化数据库：`mysql < scripts/init_db.sql`
2. 配置环境：编辑 `configs/config.yaml`
3. 启动网关：`go run api-gateway/main.go`

## 技术栈

Go 1.21 | Gin | MySQL | Redis | Kafka | gRPC | Consul | Playwright
