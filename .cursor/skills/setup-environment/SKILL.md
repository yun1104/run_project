---
name: setup-environment
description: 项目环境搭建指南，包含 MySQL/Redis 安装和建表步骤。当协作者需要搭建本项目开发环境时使用。
---

# 项目环境搭建

## 前置要求

- Windows 系统
- MySQL 8.0（本地安装）
- Redis（本地安装）
- Go 1.21+（或由启动脚本自动下载）

---

## 第一步：安装 MySQL

1. 下载 MySQL 8.0 安装包：https://dev.mysql.com/downloads/installer/
2. 安装时设置 root 密码为 `123456`
3. 确保 MySQL 服务在 `3306` 端口运行

> 密码必须是 `123456`，否则需要修改 `run_silent.vbs` 里的 `-MySQLPassword` 参数。

---

## 第二步：初始化数据库和建表

MySQL 安装完成后，执行建表脚本：

```cmd
mysql -u root -p123456 < scripts\init_db.sql
```

该脚本会创建：
- 数据库 `meituan_db_0`、`meituan_db_1`
- 表 `users`（用户账号密码）
- 表 `user_preferences_0`（用户饮食偏好）
- 表 `orders_202601`（订单记录，按月分表）

---

## 第三步：安装 Redis

1. 下载 Redis for Windows：https://github.com/tporadowski/redis/releases
2. 解压后运行 `redis-server.exe`，或注册为 Windows 服务
3. 确保 Redis 在 `6379` 端口运行（无密码）

---

## 第四步：启动项目

直接双击 `run_silent.vbs`。

启动脚本会自动：
- 检测 Go 环境（没有则自动下载便携版）
- 停止旧进程
- 运行 `go mod tidy` 安装依赖
- 启动服务并打开浏览器

服务地址：`http://127.0.0.1:8080/`

---

## 停止服务

双击 `stop_silent.vbs`，或运行：

```cmd
powershell -ExecutionPolicy Bypass -File scripts\stop.ps1
```

---

## 配置说明

| 参数 | 默认值 | 位置 |
|------|--------|------|
| MySQL 地址 | 127.0.0.1:3306 | run_silent.vbs |
| MySQL 用户名 | root | run_silent.vbs |
| MySQL 密码 | 123456 | run_silent.vbs |
| Redis 地址 | 127.0.0.1:6379 | run_silent.vbs |
| AI API Token | ms-dd4cdb20-b7a7-4e39-95ea-ae1b5f412d4d | run_silent.vbs（已内置） |

如需修改，直接编辑 `run_silent.vbs` 第50行的启动命令参数。
