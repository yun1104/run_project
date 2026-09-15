#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""通过 SSH 在远程服务器执行 Docker 部署。需: pip install paramiko
环境变量: DEPLOY_HOST, DEPLOY_USER, DEPLOY_PASSWORD"""
import os
import sys

if sys.stdout.encoding and sys.stdout.encoding.lower() != "utf-8":
    try:
        sys.stdout.reconfigure(encoding="utf-8")
    except Exception:
        pass

try:
    import paramiko
except ImportError:
    print("请安装: pip install paramiko")
    sys.exit(1)

HOST = os.environ.get("DEPLOY_HOST", "8.134.191.205")
USER = os.environ.get("DEPLOY_USER", "root")
PASSWORD = os.environ.get("DEPLOY_PASSWORD", "AIwaimaizhushou52")

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        client.connect(HOST, username=USER, password=PASSWORD, timeout=15)
        print(f"已连接 {USER}@{HOST}，开始部署...")
        cmds = [
            # 配置国内镜像加速（解决 Docker Hub 拉取超时）
            "mkdir -p /etc/containers/registries.conf.d",
            'echo -e \'[[registry]]\\nlocation = "docker.io"\\n[[registry.mirror]]\\nlocation = "docker.1ms.run"\' > /etc/containers/registries.conf.d/99-mirror.conf',
            # 启动 Podman 套接字，并建立 docker.sock 兼容（阿里云 podman-docker）
            "systemctl enable --now podman.socket 2>/dev/null || true",
            "sleep 2",
            "test -S /var/run/docker.sock || (test -S /run/podman/podman.sock && ln -sf /run/podman/podman.sock /var/run/docker.sock) || true",
            "docker info 2>&1 | head -3 || echo 'docker info check'",
            # 安装 docker-compose
            "(docker compose version 2>/dev/null || docker-compose -v 2>/dev/null) || (curl -sL https://github.com/docker/compose/releases/download/v2.24.0/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose && chmod +x /usr/local/bin/docker-compose)",
            # 确保项目目录存在
            "cd /opt && (test -d run_project || git clone -b input https://github.com/yun1104/run_project.git) && cd run_project && git pull origin input 2>/dev/null || true",
            "cd /opt/run_project && cp -f .env.example .env 2>/dev/null || true",
            # 停止旧容器后重新构建
            "cd /opt/run_project && (docker compose down 2>/dev/null || docker-compose down 2>/dev/null); docker compose up -d --build 2>/dev/null || docker-compose up -d --build",
            "sleep 30",
            "cd /opt/run_project && (docker compose ps 2>/dev/null || docker-compose ps)",
            "curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/ 2>/dev/null || echo '8080 check done'",
        ]
        for cmd in cmds:
            print(f"\n>>> {cmd}")
            stdin, stdout, stderr = client.exec_command(cmd, get_pty=True)
            for line in iter(stdout.readline, ""):
                try:
                    print(line, end="")
                except UnicodeEncodeError:
                    print(line.encode("utf-8", errors="replace").decode("utf-8"), end="")
            try:
                err = stderr.read().decode("utf-8", errors="replace")
            except Exception:
                err = ""
            if err:
                print("STDERR:", err, file=sys.stderr)
        print("\n部署完成。访问: http://8.134.191.205:8080/")
    except Exception as e:
        print(f"错误: {e}")
        sys.exit(1)
    finally:
        client.close()

if __name__ == "__main__":
    main()
