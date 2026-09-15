# 外卖推荐系统（分布式）

当前版本仅提供推荐能力，不包含下单功能。

## 当前架构

- 客户端层：`app/web`
- 网关层：`app/cmd/gateway`（HTTP入口、鉴权、限流）
- 应用层：`app/cmd/app-orchestrator`（聚合编排）
- 服务层：
  - `app/cmd/user-service`
  - `app/cmd/recommend-service`
- 服务间通信：gRPC（JSON Codec）
- 数据层：`app/pkg/database` + `app/pkg/cache`（不可用时自动降级内存）

## 项目结构

```text
run_project/
├── app/
│   ├── cmd/
│   │   ├── gateway/
│   │   ├── app-orchestrator/
│   │   ├── user-service/
│   │   └── recommend-service/
│   ├── internal/
│   ├── pkg/
│   ├── proto/
│   ├── web/
│   ├── configs/
│   ├── scripts/
│   ├── go.mod
│   └── go.sum
├── README.md
└── INTERVIEW.md
```

## 启动方式

按顺序启动：

```bash
cd app
go run ./cmd/user-service
go run ./cmd/recommend-service
go run ./cmd/app-orchestrator
go run ./cmd/gateway
```

默认端口：

- gateway: `8080`
- app-orchestrator: `50050`
- user-service: `50051`
- recommend-service: `50053`

## 接口

- `POST /api/v1/user/register`
- `POST /api/v1/user/login`
- `GET /api/v1/user/preference`
- `PUT /api/v1/user/preference`
- `POST /api/v1/recommend/get`

说明：`/api/v1/order/*` 已移除。
