#!/bin/bash
set -e
cd "$(dirname "$0")/.."

echo "=== 检查 Docker ==="
if ! command -v docker &>/dev/null; then
  echo "请先安装 Docker: https://docs.docker.com/engine/install/"
  exit 1
fi
if ! docker compose version 2>/dev/null && ! docker-compose version 2>/dev/null; then
  echo "请安装 Docker Compose"
  exit 1
fi

echo "=== 环境配置 ==="
if [ ! -f .env ]; then
  cp .env.example .env
  echo "已创建 .env，请编辑填入 MODELSCOPE_API_KEY 和 AMAP_API_KEY 后重试"
  exit 1
fi

echo "=== 构建并启动 ==="
docker compose up -d --build

echo "=== 等待服务就绪 ==="
sleep 5
docker compose ps

echo ""
echo "部署完成。访问: http://$(hostname -I 2>/dev/null | awk '{print $1}'):8080/"
echo "或: http://8.134.191.205:8080/"
