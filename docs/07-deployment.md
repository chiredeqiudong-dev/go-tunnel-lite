# 部署指南

## 1. 编译构建

### 1.1 环境要求

| 要求 | 版本 |
|------|------|
| Go | 1.21 或更高 |
| Make | 任意版本（可选） |

### 1.2 使用 Make 编译

```bash
# 克隆项目
git clone https://github.com/your-repo/go-tunnel-lite.git
cd go-tunnel-lite

# 编译所有组件
make build

# 仅编译服务端
make server

# 仅编译客户端
make client

# 编译产物位置
ls bin/
# go-tunnel-server
# go-tunnel-client
```

### 1.3 手动编译

```bash
# 编译服务端
go build -o bin/go-tunnel-server ./cmd/server

# 编译客户端
go build -o bin/go-tunnel-client ./cmd/client
```

### 1.4 交叉编译

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o bin/go-tunnel-server-linux ./cmd/server
GOOS=linux GOARCH=amd64 go build -o bin/go-tunnel-client-linux ./cmd/client

# Linux ARM64 (树莓派等)
GOOS=linux GOARCH=arm64 go build -o bin/go-tunnel-server-arm64 ./cmd/server

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/go-tunnel-server.exe ./cmd/server

# macOS
GOOS=darwin GOARCH=amd64 go build -o bin/go-tunnel-server-darwin ./cmd/server
```

### 1.5 编译优化

```bash
# 减小二进制体积
go build -ldflags "-s -w" -o bin/go-tunnel-server ./cmd/server

# -s: 去除符号表
# -w: 去除调试信息
```

## 2. 服务端部署

### 2.1 快速部署

```bash
# 1. 上传二进制文件到服务器
scp bin/go-tunnel-server user@your-server:/opt/tunnel/

# 2. 上传配置文件
scp configs/server.yaml user@your-server:/opt/tunnel/

# 3. SSH 登录服务器
ssh user@your-server

# 4. 修改配置
vim /opt/tunnel/server.yaml

# 5. 启动服务
/opt/tunnel/go-tunnel-server -c /opt/tunnel/server.yaml
```

### 2.2 配置防火墙

```bash
# Ubuntu/Debian (ufw)
sudo ufw allow 7000/tcp   # 控制端口
sudo ufw allow 8080/tcp   # 隧道端口（根据需要）
sudo ufw allow 2222/tcp   # 隧道端口（根据需要）

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=7000/tcp
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload

# iptables
sudo iptables -A INPUT -p tcp --dport 7000 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
```

### 2.3 使用 Systemd 管理

创建服务文件 `/etc/systemd/system/go-tunnel-server.service`：

```ini
[Unit]
Description=Go-Tunnel-Lite Server
After=network.target

[Service]
Type=simple
User=tunnel
Group=tunnel
WorkingDirectory=/opt/tunnel
ExecStart=/opt/tunnel/go-tunnel-server -c /opt/tunnel/server.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535

# 日志输出
StandardOutput=append:/var/log/tunnel/server.log
StandardError=append:/var/log/tunnel/server.log

[Install]
WantedBy=multi-user.target
```

**启用和管理服务**：

```bash
# 创建用户和目录
sudo useradd -r -s /bin/false tunnel
sudo mkdir -p /opt/tunnel /var/log/tunnel
sudo chown tunnel:tunnel /opt/tunnel /var/log/tunnel

# 重新加载 systemd
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start go-tunnel-server

# 开机自启
sudo systemctl enable go-tunnel-server

# 查看状态
sudo systemctl status go-tunnel-server

# 查看日志
sudo journalctl -u go-tunnel-server -f

# 重启服务
sudo systemctl restart go-tunnel-server

# 停止服务
sudo systemctl stop go-tunnel-server
```

### 2.4 使用 Supervisor 管理

创建配置文件 `/etc/supervisor/conf.d/go-tunnel-server.conf`：

```ini
[program:go-tunnel-server]
command=/opt/tunnel/go-tunnel-server -c /opt/tunnel/server.yaml
directory=/opt/tunnel
user=tunnel
autostart=true
autorestart=true
startsecs=3
startretries=3
redirect_stderr=true
stdout_logfile=/var/log/tunnel/server.log
stdout_logfile_maxbytes=50MB
stdout_logfile_backups=10
```

**管理命令**：

```bash
# 更新配置
sudo supervisorctl reread
sudo supervisorctl update

# 启动
sudo supervisorctl start go-tunnel-server

# 停止
sudo supervisorctl stop go-tunnel-server

# 重启
sudo supervisorctl restart go-tunnel-server

# 查看状态
sudo supervisorctl status
```

### 2.5 Docker 部署

**Dockerfile**：

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -ldflags "-s -w" -o go-tunnel-server ./cmd/server

FROM alpine:3.18

RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/go-tunnel-server .
COPY configs/server.yaml .

EXPOSE 7000 8080 2222

ENTRYPOINT ["./go-tunnel-server", "-c", "server.yaml"]
```

**构建和运行**：

```bash
# 构建镜像
docker build -t go-tunnel-server .

# 运行容器
docker run -d \
  --name tunnel-server \
  -p 7000:7000 \
  -p 8080:8080 \
  -p 2222:2222 \
  -v /path/to/server.yaml:/app/server.yaml \
  go-tunnel-server
```

**docker-compose.yml**：

```yaml
version: '3.8'

services:
  tunnel-server:
    build: .
    container_name: go-tunnel-server
    ports:
      - "7000:7000"
      - "8080:8080"
      - "2222:2222"
    volumes:
      - ./configs/server.yaml:/app/server.yaml:ro
    restart: always
```

## 3. 客户端部署

### 3.1 Linux 客户端

```bash
# 1. 下载或编译客户端二进制
mkdir -p /opt/tunnel
cp go-tunnel-client /opt/tunnel/
cp client.yaml /opt/tunnel/

# 2. 修改配置
vim /opt/tunnel/client.yaml

# 3. 启动
/opt/tunnel/go-tunnel-client -c /opt/tunnel/client.yaml
```

### 3.2 使用 Systemd 管理客户端

创建 `/etc/systemd/system/go-tunnel-client.service`：

```ini
[Unit]
Description=Go-Tunnel-Lite Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/tunnel
ExecStart=/opt/tunnel/go-tunnel-client -c /opt/tunnel/client.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable go-tunnel-client
sudo systemctl start go-tunnel-client
```

### 3.3 Windows 客户端

1. 下载 `go-tunnel-client.exe`
2. 创建配置文件 `client.yaml`
3. 运行：

```powershell
.\go-tunnel-client.exe -c client.yaml
```

**注册为 Windows 服务**（使用 NSSM）：

```powershell
# 下载 nssm: https://nssm.cc/
nssm install go-tunnel-client "C:\tunnel\go-tunnel-client.exe" "-c C:\tunnel\client.yaml"
nssm start go-tunnel-client
```

### 3.4 macOS 客户端

使用 launchd 管理：

创建 `~/Library/LaunchAgents/com.tunnel.client.plist`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.tunnel.client</string>
    <key>ProgramArguments</key>
    <array>
        <string>/opt/tunnel/go-tunnel-client</string>
        <string>-c</string>
        <string>/opt/tunnel/client.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/tunnel-client.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/tunnel-client.log</string>
</dict>
</plist>
```

```bash
launchctl load ~/Library/LaunchAgents/com.tunnel.client.plist
launchctl start com.tunnel.client
```

## 4. 验证部署

### 4.1 验证服务端

```bash
# 检查端口监听
ss -tlnp | grep 7000
# 或
netstat -tlnp | grep 7000

# 测试端口连通性（从其他机器）
telnet your-server-ip 7000
# 或
nc -zv your-server-ip 7000
```

### 4.2 验证客户端

```bash
# 查看客户端日志
tail -f /var/log/tunnel/client.log

# 应该看到类似输出：
# 正在连接服务端 addr=your-server:7000
# 已连接到服务端
# 认证成功
# 隧道注册成功 name=web remotePort=8080
```

### 4.3 验证隧道

```bash
# 假设配置了 web 隧道：本地 8080 → 远程 8080

# 1. 在内网机器启动一个 Web 服务
python3 -m http.server 8080

# 2. 从外网访问
curl http://your-server-ip:8080

# 应该能看到内网机器的文件列表
```

### 4.4 验证 SSH 隧道

```bash
# 假设配置了 SSH 隧道：本地 22 → 远程 2222

# 从外网 SSH 连接
ssh -p 2222 user@your-server-ip

# 应该登录到内网机器
```

## 5. 运维管理

### 5.1 日志管理

**配置日志轮转** (logrotate)：

创建 `/etc/logrotate.d/go-tunnel`：

```
/var/log/tunnel/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 tunnel tunnel
    postrotate
        systemctl reload go-tunnel-server > /dev/null 2>&1 || true
    endscript
}
```

### 5.2 监控

**检查进程**：
```bash
pgrep -f go-tunnel-server
pgrep -f go-tunnel-client
```

**监控端口**：
```bash
# 使用 curl 检查（如果有 HTTP 服务）
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080

# 使用 nc 检查端口
nc -zv localhost 7000 && echo "OK" || echo "FAIL"
```

### 5.3 故障排查

| 问题 | 可能原因 | 排查方法 |
|------|----------|----------|
| 客户端连接失败 | 防火墙、网络不通 | `telnet server 7000` |
| 认证失败 | Token 不匹配 | 检查两端配置 |
| 隧道注册失败 | 端口不在白名单 | 检查 public_ports |
| 访问超时 | 本地服务未启动 | 检查 local_addr |
| 连接断开 | 心跳超时 | 检查网络稳定性 |

**查看详细日志**：
```bash
# 临时开启 debug 日志
# 修改配置 log.level: debug
# 重启服务

# 查看实时日志
tail -f /var/log/tunnel/server.log
```

## 6. 安全加固

### 6.1 网络安全

```bash
# 仅允许特定 IP 连接控制端口
iptables -A INPUT -p tcp --dport 7000 -s trusted-ip -j ACCEPT
iptables -A INPUT -p tcp --dport 7000 -j DROP
```

### 6.2 最小权限原则

```bash
# 创建专用用户
useradd -r -s /bin/false tunnel

# 设置文件权限
chmod 700 /opt/tunnel
chmod 600 /opt/tunnel/server.yaml  # 配置文件只读
chown -R tunnel:tunnel /opt/tunnel
```

### 6.3 安全检查清单

- [ ] 使用强 Token（至少 32 字符）
- [ ] 配置端口白名单
- [ ] 开启防火墙
- [ ] 限制访问 IP（如需要）
- [ ] 定期更新程序
- [ ] 监控异常连接
- [ ] 定期检查日志

## 7. 性能调优

### 7.1 系统参数

```bash
# /etc/sysctl.conf

# 增加最大文件描述符
fs.file-max = 2097152

# TCP 调优
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.tcp_fin_timeout = 30
net.ipv4.tcp_keepalive_time = 1200

# 应用配置
sysctl -p
```

### 7.2 进程限制

```bash
# /etc/security/limits.conf
tunnel soft nofile 65535
tunnel hard nofile 65535
```

### 7.3 服务配置

```ini
# systemd 服务文件中添加
LimitNOFILE=65535
```

## 8. 升级指南

### 8.1 升级步骤

1. **备份配置**
   ```bash
   cp /opt/tunnel/server.yaml /opt/tunnel/server.yaml.bak
   ```

2. **停止服务**
   ```bash
   systemctl stop go-tunnel-server
   ```

3. **替换二进制**
   ```bash
   cp new-go-tunnel-server /opt/tunnel/go-tunnel-server
   ```

4. **检查配置兼容性**
   - 查看更新日志
   - 检查配置格式变化

5. **启动服务**
   ```bash
   systemctl start go-tunnel-server
   ```

6. **验证**
   ```bash
   systemctl status go-tunnel-server
   tail -f /var/log/tunnel/server.log
   ```

### 8.2 回滚

```bash
# 停止服务
systemctl stop go-tunnel-server

# 恢复旧版本
cp /opt/tunnel/go-tunnel-server.bak /opt/tunnel/go-tunnel-server

# 恢复配置
cp /opt/tunnel/server.yaml.bak /opt/tunnel/server.yaml

# 启动
systemctl start go-tunnel-server
```
