# 部署指南

## 环境要求

- Go 1.21+
- Docker & Docker Compose
- MySQL 8.0+
- Redis 7+
- Kafka 3.0+

## 快速部署

### 1. 启动基础设施

```bash
docker-compose -f scripts/docker-compose.yml up -d
```

启动组件：
- MySQL (3306, 3307)
- Redis Cluster (7000, 7001, 7002)
- Kafka (9092)
- Consul (8500)
- Prometheus (9090)
- Grafana (3000)
- Jaeger (16686)

### 2. 初始化数据库

```bash
mysql -h 127.0.0.1 -P 3306 -u root -proot < scripts/init_db.sql
```

### 3. 配置环境

编辑 `configs/config.yaml`，填入正确的配置：
- AI API密钥
- 代理IP列表（如需要）

### 4. 启动服务

```bash
# API网关
go run api-gateway/main.go

# 用户服务
go run services/user-service/main.go

# 订单服务
go run services/order-service/main.go

# 推荐服务
go run services/recommend-service/main.go

# 其他服务...
```

## 监控访问

- **Consul**: http://localhost:8500 - 服务注册与发现
- **Prometheus**: http://localhost:9090 - 指标监控
- **Grafana**: http://localhost:3000 (admin/admin) - 可视化监控
- **Jaeger**: http://localhost:16686 - 链路追踪

## 性能测试

使用 wrk 或 ab 进行压测：

```bash
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/recommend/get
```

## 扩容方案

- 网关：Nginx负载均衡
- 服务：Kubernetes HPA自动扩容
- 数据库：增加分库数量
- 缓存：扩展Redis Cluster节点

## 故障排查

查看服务日志、Prometheus指标、Jaeger链路追踪定位问题。
