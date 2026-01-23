# 配置说明

## 1. 配置概述

Go-Tunnel-Lite 使用 YAML 格式的配置文件，分为服务端配置和客户端配置。

### 1.1 配置文件位置

| 组件 | 默认配置文件 | 命令行指定 |
|------|-------------|-----------|
| 服务端 | server.yaml | `-c /path/to/server.yaml` |
| 客户端 | client.yaml | `-c /path/to/client.yaml` |

### 1.2 配置加载流程

```
命令行参数 -c → 读取文件 → YAML 解析 → 配置验证 → 应用默认值
```

## 2. 服务端配置

### 2.1 完整配置示例

```yaml
# Go-Tunnel-Lite 服务端配置

server:
  # 控制端口地址，客户端连接此端口
  control_addr: "0.0.0.0:7000"
  
  # 认证令牌，需要与客户端配置一致
  token: "my-secret-token"
  
  # 心跳间隔
  heartbeat_interval: 30s
  
  # 心跳超时，超过此时间未收到心跳则断开连接
  heartbeat_timeout: 90s
  
  # 允许客户端使用的公共端口白名单
  # 为空则允许所有端口
  public_ports:
    - 8080   # Web 服务
    - 2222   # SSH 服务
    - 3306   # MySQL

# 日志配置
log:
  # 日志级别: debug, info, warn, error
  level: "info"
  
  # 日志文件路径，为空则输出到控制台
  file: ""
```

### 2.2 配置项详解

#### server.control_addr

| 属性 | 值 |
|------|-----|
| 类型 | string |
| 必填 | 是 |
| 默认值 | 无 |
| 格式 | `host:port` |

**说明**：控制端口监听地址，客户端连接此端口进行认证和注册隧道。

**示例**：
```yaml
# 监听所有网卡的 7000 端口
control_addr: "0.0.0.0:7000"

# 仅监听本地（用于测试）
control_addr: "127.0.0.1:7000"

# 监听特定网卡
control_addr: "192.168.1.100:7000"
```

**设计理由**：
- 使用独立的控制端口，与数据端口分离
- 默认 7000 端口，与常见服务不冲突
- 支持绑定特定 IP，增强安全性

---

#### server.token

| 属性 | 值 |
|------|-----|
| 类型 | string |
| 必填 | 是 |
| 默认值 | 无 |
| 安全建议 | 至少 16 个字符 |

**说明**：认证令牌，客户端必须提供匹配的 Token 才能连接。

**示例**：
```yaml
# 简单 Token（仅用于测试）
token: "test-token"

# 安全 Token（生产环境）
token: "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"

# 使用环境变量（推荐）
# 配置文件中不写 token，通过环境变量传入
```

**安全建议**：
1. 使用长度足够的随机字符串
2. 不要在代码仓库中提交真实 Token
3. 生产环境使用环境变量或密钥管理服务

---

#### server.heartbeat_interval

| 属性 | 值 |
|------|-----|
| 类型 | duration |
| 必填 | 否 |
| 默认值 | 30s |
| 单位 | s(秒), m(分钟), h(小时) |

**说明**：服务端向客户端发送心跳（Ping）的间隔。

**示例**：
```yaml
heartbeat_interval: 30s    # 30 秒
heartbeat_interval: 1m     # 1 分钟
heartbeat_interval: 300s   # 5 分钟
```

**设计理由**：
- 30 秒是较好的平衡点
- 太短：增加网络开销
- 太长：故障检测延迟

---

#### server.heartbeat_timeout

| 属性 | 值 |
|------|-----|
| 类型 | duration |
| 必填 | 否 |
| 默认值 | 90s |
| 建议 | 至少 heartbeat_interval × 2 |

**说明**：心跳超时时间，超过此时间未收到客户端消息则断开连接。

**示例**：
```yaml
heartbeat_timeout: 90s     # 90 秒
heartbeat_timeout: 2m      # 2 分钟
heartbeat_timeout: 180s    # 3 分钟
```

**设计理由**：
- 设为心跳间隔的 3 倍（90s = 30s × 3）
- 容忍 1-2 次心跳丢失
- 网络抖动不会立即断开

---

#### server.public_ports

| 属性 | 值 |
|------|-----|
| 类型 | []int |
| 必填 | 否 |
| 默认值 | [] (允许所有) |
| 范围 | 1-65535 |

**说明**：允许客户端使用的端口白名单。为空时允许所有端口。

**示例**：
```yaml
# 允许指定端口
public_ports:
  - 8080
  - 8081
  - 2222
  - 3306

# 允许所有端口（开发环境）
public_ports: []

# 或直接不写这个配置项
```

**设计理由**：
- **安全控制**：防止客户端占用敏感端口（如 22、80、443）
- **资源管理**：限制可用端口范围
- **灵活性**：空列表允许所有端口，方便开发测试

---

#### log.level

| 属性 | 值 |
|------|-----|
| 类型 | string |
| 必填 | 否 |
| 默认值 | info |
| 可选值 | debug, info, warn, error |

**说明**：日志级别，控制输出的日志详细程度。

**级别说明**：
| 级别 | 说明 | 使用场景 |
|------|------|----------|
| debug | 最详细，包含调试信息 | 开发调试 |
| info | 一般信息 | 正常运行 |
| warn | 警告信息 | 需要关注 |
| error | 错误信息 | 问题排查 |

---

#### log.file

| 属性 | 值 |
|------|-----|
| 类型 | string |
| 必填 | 否 |
| 默认值 | "" (控制台) |

**说明**：日志文件路径，为空则输出到控制台。

**示例**：
```yaml
# 输出到文件
log:
  file: "/var/log/tunnel/server.log"

# 输出到控制台（默认）
log:
  file: ""
```

## 3. 客户端配置

### 3.1 完整配置示例

```yaml
# Go-Tunnel-Lite 客户端配置

client:
  # 服务端地址
  server_addr: "your-server-ip:7000"
  
  # 认证令牌，需要与服务端配置一致
  token: "my-secret-token"
  
  # 心跳间隔
  heartbeat_interval: 30s
  
  # 隧道配置列表
  tunnels:
    # Web 服务隧道
    - name: "web"
      local_addr: "127.0.0.1:8080"
      remote_port: 8080
    
    # SSH 隧道
    - name: "ssh"
      local_addr: "127.0.0.1:22"
      remote_port: 2222
    
    # MySQL 隧道
    - name: "mysql"
      local_addr: "127.0.0.1:3306"
      remote_port: 3306

# 日志配置
log:
  level: "info"
  file: ""
```

### 3.2 配置项详解

#### client.server_addr

| 属性 | 值 |
|------|-----|
| 类型 | string |
| 必填 | 是 |
| 格式 | `host:port` |

**说明**：服务端控制端口地址。

**示例**：
```yaml
# 使用 IP 地址
server_addr: "123.45.67.89:7000"

# 使用域名
server_addr: "tunnel.example.com:7000"

# 本地测试
server_addr: "127.0.0.1:7000"
```

---

#### client.token

| 属性 | 值 |
|------|-----|
| 类型 | string |
| 必填 | 是 |

**说明**：认证令牌，必须与服务端配置一致。

---

#### client.heartbeat_interval

| 属性 | 值 |
|------|-----|
| 类型 | duration |
| 必填 | 否 |
| 默认值 | 30s |

**说明**：客户端发送心跳的间隔。

**建议**：与服务端保持一致或稍短。

---

#### client.tunnels

| 属性 | 值 |
|------|-----|
| 类型 | []TunnelConfig |
| 必填 | 是（至少一个） |

**说明**：隧道配置列表，每个隧道定义一个穿透规则。

---

#### tunnels[].name

| 属性 | 值 |
|------|-----|
| 类型 | string |
| 必填 | 是 |
| 唯一性 | 同一客户端内唯一 |

**说明**：隧道名称，用于标识和日志。

**命名建议**：
- 使用有意义的名称：`web`、`ssh`、`api`
- 避免特殊字符
- 保持简洁

---

#### tunnels[].local_addr

| 属性 | 值 |
|------|-----|
| 类型 | string |
| 必填 | 是 |
| 格式 | `host:port` |

**说明**：内网服务的地址端口。

**示例**：
```yaml
# 本机服务
local_addr: "127.0.0.1:8080"

# 同网段其他机器
local_addr: "192.168.1.100:3306"

# 使用 localhost
local_addr: "localhost:22"
```

**注意**：确保客户端可以访问该地址。

---

#### tunnels[].remote_port

| 属性 | 值 |
|------|-----|
| 类型 | int |
| 必填 | 是 |
| 范围 | 1-65535 |

**说明**：在服务端暴露的公网端口。

**注意**：
- 必须在服务端的 `public_ports` 白名单中（如果配置了白名单）
- 确保服务端防火墙已开放该端口

## 4. 配置验证

### 4.1 服务端配置验证

```go
func (c *ServerConfig) Validate() error {
    if c.Server.ControlAddr == "" {
        return fmt.Errorf("server.control_addr is required")
    }
    if c.Server.Token == "" {
        return fmt.Errorf("server.token is required")
    }
    // 设置默认值
    if c.Server.HeartbeatInterval <= 0 {
        c.Server.HeartbeatInterval = 30 * time.Second
    }
    if c.Server.HeartbeatTimeout <= 0 {
        c.Server.HeartbeatTimeout = 90 * time.Second
    }
    return nil
}
```

### 4.2 客户端配置验证

```go
func (c *ClientConfig) Validate() error {
    if c.Client.ServerAddr == "" {
        return fmt.Errorf("client.server_addr is required")
    }
    if c.Client.Token == "" {
        return fmt.Errorf("client.token is required")
    }
    if len(c.Client.Tunnels) == 0 {
        return fmt.Errorf("client.tunnels is required, at least one tunnel")
    }
    // 验证每个隧道
    for i, t := range c.Client.Tunnels {
        if t.Name == "" {
            return fmt.Errorf("tunnel[%d].name is required", i)
        }
        if t.LocalAddr == "" {
            return fmt.Errorf("tunnel[%d].local_addr is required", i)
        }
        if t.RemotePort <= 0 || t.RemotePort > 65535 {
            return fmt.Errorf("tunnel[%d].remote_port must be between 1 and 65535", i)
        }
    }
    return nil
}
```

## 5. 配置最佳实践

### 5.1 开发环境配置

**服务端** (server.yaml)：
```yaml
server:
  control_addr: "0.0.0.0:7000"
  token: "dev-token"
  heartbeat_interval: 30s
  heartbeat_timeout: 90s
  public_ports: []  # 允许所有端口

log:
  level: "debug"
```

**客户端** (client.yaml)：
```yaml
client:
  server_addr: "127.0.0.1:7000"
  token: "dev-token"
  heartbeat_interval: 30s
  tunnels:
    - name: "web"
      local_addr: "127.0.0.1:3000"
      remote_port: 8080

log:
  level: "debug"
```

### 5.2 生产环境配置

**服务端** (server.yaml)：
```yaml
server:
  control_addr: "0.0.0.0:7000"
  token: "your-very-long-secure-random-token-here"
  heartbeat_interval: 30s
  heartbeat_timeout: 90s
  public_ports:
    - 8080
    - 8443
    - 2222

log:
  level: "info"
  file: "/var/log/tunnel/server.log"
```

**客户端** (client.yaml)：
```yaml
client:
  server_addr: "tunnel.yourdomain.com:7000"
  token: "your-very-long-secure-random-token-here"
  heartbeat_interval: 30s
  tunnels:
    - name: "web"
      local_addr: "127.0.0.1:80"
      remote_port: 8080
    - name: "ssh"
      local_addr: "127.0.0.1:22"
      remote_port: 2222

log:
  level: "info"
  file: "/var/log/tunnel/client.log"
```

### 5.3 安全建议

| 建议 | 说明 |
|------|------|
| 使用强 Token | 随机生成，至少 32 个字符 |
| 限制端口白名单 | 只开放必要的端口 |
| 配置防火墙 | 服务端只开放控制端口和隧道端口 |
| 定期轮换 Token | 增强安全性 |
| 使用 TLS | 考虑在协议层添加加密（未来功能） |

## 6. 常见问题

### Q: 配置文件放在哪里？

默认情况下，程序在当前目录查找配置文件。推荐：
- 开发环境：`./configs/server.yaml`
- 生产环境：`/etc/tunnel/server.yaml`

### Q: 如何使用环境变量？

当前版本不支持环境变量替换。建议：
1. 使用配置管理工具生成配置文件
2. 或在启动脚本中动态生成

### Q: Token 忘记了怎么办？

Token 不会保存在任何地方，如果忘记：
1. 修改服务端配置，设置新 Token
2. 重启服务端
3. 更新所有客户端配置
4. 重启客户端

### Q: 配置修改后需要重启吗？

是的，配置文件在启动时加载，修改后需要重启程序生效。
