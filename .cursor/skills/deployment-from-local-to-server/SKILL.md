---
name: deployment-from-local-to-server
description: 将本地代码部署到 Linux 服务器并验证可用性的标准流程。用于用户提到“部署到服务器”“发布上线”“Docker 部署”“阿里云 ECS 部署”“部署后验证”时，指导从本地构建、上传、启动、联通测试到故障排查的完整步骤。
---

# 本地到服务器部署（Docker）

## 适用场景

- 本地是 Windows 开发机，服务器是 Linux
- 目标是在服务器运行 `run_project`，并可公网访问
- 使用 Docker/Podman + Compose

## 一次部署的标准流程

复制以下清单并按顺序执行：

```text
部署清单:
- [ ] 1. 本地代码准备（分支/提交）
- [ ] 2. 本地构建 Linux 网关二进制
- [ ] 3. 上传部署文件到服务器
- [ ] 4. 服务器启动容器
- [ ] 5. 服务健康检查
- [ ] 6. 公网连通性检查
- [ ] 7. 结果确认与回归测试
```

## 1) 本地代码准备

在项目根目录执行：

```powershell
git checkout input
git pull origin input
git status
```

如有部署相关改动，先提交并推送。

## 2) 本地构建 Linux 二进制

为了避免服务器上 `go mod download` 超时，优先采用预编译方式：

```powershell
$env:GOPROXY="https://goproxy.cn,direct"
$env:GOOS="linux"
$env:CGO_ENABLED="0"
go build -o gateway_linux ./api-gateway
```

产物：`gateway_linux`（项目根目录）。

## 3) 上传部署文件到服务器

必传文件：

- `gateway_linux`
- `Dockerfile.gateway`
- `docker-compose.prebuilt.yml`
- `docker-compose.yml`
- `services/ai-service/python-recommend/Dockerfile`
- `services/ai-service/python-recommend/app.py`
- `services/ai-service/python-recommend/requirements.txt`

可选方式：

- 使用 `scripts/remote_deploy.py` 自动上传并部署
- 或手动 `scp/sftp` 上传到 `/opt/run_project/`

## 4) 服务器启动容器

登录服务器后执行：

```bash
cd /opt/run_project
cp -f .env.example .env

# 推荐：预编译模式
docker compose -f docker-compose.prebuilt.yml down
docker compose -f docker-compose.prebuilt.yml up -d --build
```

若是 Podman 环境，先启用 socket：

```bash
systemctl enable --now podman.socket || true
test -S /var/run/docker.sock || ln -sf /run/podman/podman.sock /var/run/docker.sock || true
```

## 5) 服务健康检查

服务器内检查：

```bash
docker compose -f docker-compose.prebuilt.yml ps
ss -lntup | grep -E "8080|9091|9092|3306|6379"
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://127.0.0.1:8080/
```

通过标准：

- `api-gateway/mysql/redis/python-recommend` 都是 `Up`
- `127.0.0.1:8080` 返回 `HTTP 200`

## 6) 公网连通性检查

从本地开发机检查：

```powershell
Test-NetConnection <服务器IP> -Port 8080
curl.exe --max-time 12 "http://<服务器IP>:8080/"
```

如果服务器内正常但公网不通，优先排查：

1. 云安全组未放行 `8080`
2. 云防火墙拦截
3. 运营商或机房 ACL 限制

## 7) 结果确认与回归

至少验证：

- 首页能打开
- 登录/注册可用
- 定位功能逻辑正常（注意：公网 IP + HTTP 下浏览器地理定位会受限）
- 推荐接口可返回数据

## 定位功能特别说明

浏览器地理定位 API 需要安全上下文：

- `https://域名` 可用
- `http://localhost` 可用
- `http://公网IP:端口` 常不可用

部署后若“无法获取定位”，先判断是否因为访问方式是公网 IP + HTTP。

## 常见故障与处理

### A. 镜像拉取超时

- 使用国内镜像前缀：`docker.1ms.run/library/...`
- 或配置镜像加速到 `docker.1ms.run`

### B. `docker compose` 提示 daemon 不可用

- Podman 环境启用 `podman.socket`
- 校验 `/var/run/docker.sock` 指向 `/run/podman/podman.sock`

### C. 容器 `Up` 但公网访问超时

- 服务器内 `curl 127.0.0.1:8080` 若 200，则应用无问题
- 去云平台放行安全组入站端口（至少 8080）

### D. Python 服务构建失败（找不到 `app.py`）

- 确认上传了：
  - `services/ai-service/python-recommend/app.py`
  - `services/ai-service/python-recommend/requirements.txt`
  - `services/ai-service/python-recommend/Dockerfile`

## 默认输出要求

部署任务完成时，输出必须包含：

1. 服务器内检查结果（`ps`、`curl 127.0.0.1`）
2. 公网检查结果（`Test-NetConnection`/`curl`）
3. 若失败，明确失败层级（应用层/主机层/云网络层）和下一步动作
