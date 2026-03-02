---
name: setup-environment
description: 项目环境搭建指南，包含所有依赖安装步骤。当协作者需要搭建本项目开发环境、安装依赖、配置数据库/Redis/Kafka等基础设施时使用。
---

# 项目环境搭建

## 前置要求

- Go 1.21+
- Docker & Docker Compose
- Git

## 第一步：克隆项目

```bash
git clone <repo-url>
cd run_project
```

## 第二步：安装 Go 依赖

```bash
go mod tidy
```

安装 Playwright 浏览器（爬虫功能需要）：

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps
```

## 第三步：配置环境变量

复制并填写 `.env`：

```bash
cp .env.example .env  # 如没有 example 则手动创建
```

`.env` 内容：

```
MODELSCOPE_API_KEY=your-api-key-here
```

> API Key 从 [ModelScope](https://modelscope.cn) 或阿里云 DashScope 获取。

## 第四步：启动基础设施

```bash
cd scripts
docker-compose up -d
```

等待约 30 秒，待所有容器启动完成。

## 第五步：初始化数据库

```bash
docker exec -i scripts-mysql-0-1 mysql -uroot -proot < scripts/init_db.sql
```

或通过部署脚本一键完成（Linux/Mac）：

```bash
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

## 第六步：初始化 Kafka Topics

```bash
# order.history
docker exec scripts-kafka-1 kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic order.history --partitions 3 --replication-factor 1

# order.create
docker exec scripts-kafka-1 kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic order.create --partitions 3 --replication-factor 1

# order.paid
docker exec scripts-kafka-1 kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic order.paid --partitions 3 --replication-factor 1
```

## 第七步：初始化 Redis Cluster

```bash
docker exec -it scripts-redis-node-1-1 redis-cli \
  --cluster create \
  127.0.0.1:7000 127.0.0.1:7001 127.0.0.1:7002 \
  --cluster-replicas 0 --cluster-yes
```

## 第八步：修改配置文件

查看并按需修改 `configs/config.yaml`，默认配置如下：

| 服务 | 地址 | 备注 |
|------|------|------|
| MySQL-0 | 127.0.0.1:3306 | dbname: meituan_db_0 |
| MySQL-1 | 127.0.0.1:3307 | dbname: meituan_db_1 |
| Redis Cluster | 127.0.0.1:7000~7002 | 3节点无副本 |
| Kafka | 127.0.0.1:9092 | |
| Consul | 127.0.0.1:8500 | |

## 第九步：启动服务

```bash
go run main.go
```

服务默认端口：`8080`

## 基础设施服务列表

| 服务 | 端口 | 用途 |
|------|------|------|
| MySQL-0 | 3306 | 主数据库 |
| MySQL-1 | 3307 | 分库 |
| Redis (Cluster) | 7000-7002 | 缓存 |
| Kafka | 9092 | 消息队列 |
| Zookeeper | 2181 | Kafka 依赖 |
| Consul | 8500 | 服务发现 |
| Prometheus | 9090 | 监控采集 |
| Grafana | 3000 | 监控面板 (admin/admin) |
| Jaeger | 16686 | 链路追踪 |

## 常见问题

**Redis Cluster 连接失败**：确认所有3个节点容器都在运行，并已完成 cluster create 初始化。

**Kafka topics 创建失败**：等待 Zookeeper + Kafka 完全启动后再执行（约30s）。

**Playwright 浏览器缺失**：重新运行 `playwright install --with-deps`。
