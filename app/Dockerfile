# Stage 1: Build
FROM docker.1ms.run/library/golang:1.21-alpine AS builder
WORKDIR /build

# 安装 git（go mod 需要）
RUN apk add --no-cache git

# 复制 go.mod go.sum 并下载依赖
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

# 复制源码并编译
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/gateway

# Stage 2: Runtime
FROM docker.1ms.run/library/alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /gateway /app/gateway
COPY web /app/web

EXPOSE 8080 9091 9092
ENTRYPOINT ["/app/gateway"]
