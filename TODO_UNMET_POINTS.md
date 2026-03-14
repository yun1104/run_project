# 目前未做到的点

## 1) 分层功能隔离（未完全做到）
- 网关进程中仍包含跨层职责：不仅做网关路由/鉴权/限流，还直接处理聊天入库、位置存储、Python 推荐调用等业务与数据访问逻辑。
- 网关未使用 Nginx 独立进程，当前是 `app/cmd/gateway/main.go` 内置 HTTP 网关。

## 2) 应用层 Kafka 削峰（未做到）
- 当前请求链路为同步调用：前端 -> gateway -> app-orchestrator(gRPC) -> user/recommend(gRPC)。
- 项目虽有 `app/pkg/mq/kafka.go`，但主链路未接入 Kafka 进行削峰填谷与异步解耦。

## 3) 数据层“Redis 一级 + MySQL 二级缓存”（未完全做到）
- 现状主要是 Redis + 进程内内存结构 + MySQL 持久化。
- 未形成严格意义上的“MySQL 二级缓存”分层缓存模型（MySQL 当前更像持久层而非缓存层）。

## 4) 目录隔离（基本做到，但可继续细化）
- 已有分进程入口目录：`app/cmd/gateway`、`app/cmd/app-orchestrator`、`app/cmd/user-service`、`app/cmd/recommend-service`。
- 但服务实现仍集中在 `app/internal/distributed/service` 下，按进程/域进一步拆分后可提升隔离度与可维护性。
