# 共享模块详解

本文档详细介绍 `internal/pkg` 目录下的共享模块，包括设计理由和实现细节。

## 1. 模块概览

| 模块 | 路径 | 职责 |
|------|------|------|
| **config** | `pkg/config/` | 配置文件加载与验证 |
| **connect** | `pkg/connect/` | TCP 连接封装 |
| **log** | `pkg/log/` | 日志记录 |
| **proto** | `pkg/proto/` | 通信协议定义 |

## 2. config 模块

### 2.1 模块职责

- 加载 YAML 配置文件
- 解析配置到结构体
- 验证配置有效性
- 设置默认值

### 2.2 核心结构

```go
// ServerConfig 服务端配置
type ServerConfig struct {
    Server ServerSettings `yaml:"server"`
}

// ServerSettings 服务端详细设置
type ServerSettings struct {
    ControlAddr       string        `yaml:"control_addr"`
    Token             string        `yaml:"token"`
    HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
    HeartbeatTimeout  time.Duration `yaml:"heartbeat_timeout"`
    LogLevel          string        `yaml:"log_level"`
    PublicPorts       []int         `yaml:"public_ports"`
}

// ClientConfig 客户端配置
type ClientConfig struct {
    Client ClientSettings `yaml:"client"`
}

// ClientSettings 客户端详细设置
type ClientSettings struct {
    ServerAddr        string         `yaml:"server_addr"`
    Token             string         `yaml:"token"`
    HeartbeatInterval time.Duration  `yaml:"heartbeat_interval"`
    LogLevel          string         `yaml:"log_level"`
    Tunnels           []TunnelConfig `yaml:"tunnels"`
}

// TunnelConfig 单个隧道配置
type TunnelConfig struct {
    Name       string `yaml:"name"`
    LocalAddr  string `yaml:"local_addr"`
    RemotePort int    `yaml:"remote_port"`
}
```

### 2.3 设计理由

**为什么使用 YAML？**
- 人类友好，易于阅读和编辑
- 支持注释，方便说明配置项
- Go 生态有成熟的解析库（gopkg.in/yaml.v3）

**为什么配置分层（Server/Client → Settings）？**
```yaml
server:      # 第一层：组件类型
  control_addr: ...  # 第二层：具体配置
```
- 配置文件结构清晰
- 便于未来添加其他顶层配置（如 log、metrics）
- 与常见配置规范一致

**为什么使用 time.Duration？**
```yaml
heartbeat_interval: 30s  # 直接写人类可读格式
```
- YAML 解析器自动转换
- 配置更直观
- 避免手动计算秒数

### 2.4 加载流程

```go
func LoadServerConfig(path string) (*ServerConfig, error) {
    // 1. 读取文件
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }

    // 2. YAML 解析
    var config ServerConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse config file: %w", err)
    }

    // 3. 验证配置
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }

    return &config, nil
}
```

### 2.5 验证逻辑

```go
func (c *ServerConfig) Validate() error {
    // 必填字段检查
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

**验证设计原则**：
1. 必填字段必须有值
2. 可选字段提供合理默认值
3. 数值范围检查（如端口 1-65535）
4. 错误消息明确指出问题字段

## 3. connect 模块

### 3.1 模块职责

- 封装原生 TCP 连接
- 提供消息级别的读写
- 处理并发安全
- 管理连接超时

### 3.2 核心结构

```go
type Connect struct {
    conn net.Conn      // 底层 TCP 连接

    writeMu sync.Mutex // 写锁
    readMu  sync.Mutex // 读锁

    closed   bool          // 关闭标记
    closedMu sync.Mutex    // 保护关闭标记
}
```

### 3.3 设计理由

**为什么需要封装 net.Conn？**

原生 `net.Conn` 是字节流，没有消息边界概念：
```go
// 原生方式：需要自己处理消息边界
conn.Write([]byte("hello"))
conn.Write([]byte("world"))
// 对方可能收到 "helloworld" 或 "hell" + "oworld"
```

封装后：
```go
// 封装方式：消息完整性保证
conn.WriteMessage(&Message{Type: TypePing})
// 对方一定收到完整的 Ping 消息
```

**为什么需要读写锁？**

多协程可能同时操作连接：
- 消息循环在读取
- 心跳循环在发送

```go
// 无锁：可能导致数据混乱
go conn.WriteMessage(msg1)  // 协程A
go conn.WriteMessage(msg2)  // 协程B
// 结果可能是 msg1 和 msg2 的字节交错
```

使用互斥锁保证原子性：
```go
func (c *Connect) WriteMessage(msg *proto.Message) error {
    c.writeMu.Lock()
    defer c.writeMu.Unlock()
    _, err := msg.WriteTo(c.conn)
    return err
}
```

**为什么读写分别加锁？**
- TCP 是全双工，读写可以同时进行
- 单独加锁提高并发性能
- 避免读操作阻塞写操作

### 3.4 关键方法

#### ReadMessage

```go
func (c *Connect) ReadMessage() (*proto.Message, error) {
    c.readMu.Lock()
    defer c.readMu.Unlock()

    msg := &proto.Message{}
    _, err := msg.ReadFrom(c.conn)
    if err != nil {
        return nil, err
    }
    return msg, nil
}
```

**阻塞特性**：
- 此方法会阻塞直到读取完整消息
- 或发生错误（超时、连接关闭）

#### WriteMessage

```go
func (c *Connect) WriteMessage(msg *proto.Message) error {
    c.writeMu.Lock()
    defer c.writeMu.Unlock()
    _, err := msg.WriteTo(c.conn)
    return err
}
```

#### Close

```go
func (c *Connect) Close() error {
    c.closedMu.Lock()
    defer c.closedMu.Unlock()

    if c.closed {
        return nil  // 幂等性：重复关闭不报错
    }
    c.closed = true
    return c.conn.Close()
}
```

**幂等设计**：
- 多次调用 Close 不会 panic
- 第一次关闭执行实际操作
- 后续调用直接返回

#### 超时设置

```go
func (c *Connect) SetDeadline(t time.Time) error {
    return c.conn.SetDeadline(t)
}

func (c *Connect) SetReadDeadLine(t time.Time) error {
    return c.conn.SetReadDeadline(t)
}

func (c *Connect) SetWriteDeadLine(t time.Time) error {
    return c.conn.SetWriteDeadline(t)
}
```

**超时应用场景**：
- 认证超时：10秒
- 读取超时：心跳超时 + 缓冲

#### RawConn

```go
func (c *Connect) RawConn() net.Conn {
    return c.conn
}
```

**何时使用？**
- 数据转发场景需要直接操作底层连接
- 绕过消息封装，直接传输原始字节

## 4. log 模块

### 4.1 模块职责

- 提供统一的日志接口
- 支持不同日志级别
- 支持结构化日志
- 支持输出格式切换

### 4.2 核心实现

```go
// 日志级别别名
const (
    LevelDebug = slog.LevelDebug
    LevelInfo  = slog.LevelInfo
    LevelWarn  = slog.LevelWarn
    LevelError = slog.LevelError
)

// 全局日志实例
var logger *slog.Logger

// 初始化
func init() {
    handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: LevelDebug,
    })
    logger = slog.New(handler)
}
```

### 4.3 设计理由

**为什么使用 slog？**
- Go 1.21 标准库，无需第三方依赖
- 结构化日志，便于解析和查询
- 性能优秀，适合高并发场景

**为什么使用全局 logger？**
- 简化调用：直接 `log.Info(...)` 而非传递 logger
- 统一配置：一处设置，全局生效
- 符合常见日志库习惯

### 4.4 日志方法

```go
// 基础日志方法
func Debug(msg string, args ...any) {
    logger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
    logger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
    logger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
    logger.Error(msg, args...)
}
```

**结构化日志使用**：
```go
// 普通字符串拼接（不推荐）
log.Info(fmt.Sprintf("连接来自 %s", addr))

// 结构化日志（推荐）
log.Info("新连接", "addr", addr, "clientID", id)
// 输出：time=2024-01-23T10:00:00.000Z level=INFO msg="新连接" addr=192.168.1.100:12345 clientID=client-123
```

### 4.5 配置方法

```go
// 设置日志级别
func SetLevel(level slog.Level) {
    handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: level,
    })
    logger = slog.New(handler)
}

// 切换为 JSON 输出
func SetJSONOutput(level slog.Level) {
    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: level,
    })
    logger = slog.New(handler)
}
```

**文本输出**（开发环境）：
```
time=2024-01-23T10:00:00.000Z level=INFO msg="服务端启动" addr=0.0.0.0:7000
```

**JSON 输出**（生产环境）：
```json
{"time":"2024-01-23T10:00:00.000Z","level":"INFO","msg":"服务端启动","addr":"0.0.0.0:7000"}
```

### 4.6 高级功能

```go
// 带固定属性的子 Logger
func With(args ...any) *slog.Logger {
    return logger.With(args...)
}

// 示例：为特定客户端创建 logger
clientLogger := log.With("clientID", "client-123")
clientLogger.Info("处理请求")  // 自动带上 clientID
```

## 5. proto 模块

### 5.1 模块职责

- 定义消息结构
- 实现消息编解码
- 定义消息类型常量
- 定义协议常量和错误

### 5.2 消息结构

```go
type Message struct {
    Type uint8   // 消息类型
    Data []byte  // 消息体（JSON 编码）
}
```

### 5.3 消息类型

```go
const (
    // 认证相关 (0x01-0x0F)
    TypeAuth     uint8 = 0x01
    TypeAuthResp uint8 = 0x02

    // 隧道管理 (0x10-0x1F)
    TypeRegisterTunnel     uint8 = 0x10
    TypeRegisterTunnelResp uint8 = 0x11

    // 代理请求 (0x20-0x2F)
    TypeNewProxy   uint8 = 0x20
    TypeProxyReady uint8 = 0x21

    // 心跳保活 (0x30-0x3F)
    TypePing uint8 = 0x30
    TypePong uint8 = 0x31
)
```

### 5.4 设计理由

**为什么消息类型使用分段？**
- 0x01-0x0F：认证相关
- 0x10-0x1F：隧道管理
- 0x20-0x2F：代理请求
- 0x30-0x3F：心跳保活

好处：
- 看到类型值就知道所属分类
- 每个分类预留扩展空间
- 便于调试和问题定位

### 5.5 编解码实现

#### WriteTo（编码）

```go
func (m *Message) WriteTo(w io.Writer) (n int64, err error) {
    // 1. 检查消息长度
    dataLen := len(m.Data)
    if dataLen > MaxDataLen {
        return 0, ErrMsgTooLarge
    }

    // 2. 构造消息头（5字节）
    header := make([]byte, HeaderLen)
    header[0] = m.Type
    binary.BigEndian.PutUint32(header[1:5], uint32(dataLen))

    // 3. 写入消息头
    written, err := w.Write(header)
    n = int64(written)
    if err != nil {
        return n, err
    }

    // 4. 写入消息体
    if dataLen > 0 {
        written, err = w.Write(m.Data)
        n += int64(written)
    }

    return n, err
}
```

#### ReadFrom（解码）

```go
func (m *Message) ReadFrom(r io.Reader) (n int64, err error) {
    // 1. 读取消息头（5字节）
    header := make([]byte, HeaderLen)
    readN, err := io.ReadFull(r, header)
    n = int64(readN)
    if err != nil {
        return n, err
    }

    // 2. 解析消息头
    m.Type = header[0]
    dataLen := binary.BigEndian.Uint32(header[1:5])

    // 3. 检查长度合法性
    if dataLen > MaxDataLen {
        return n, ErrMsgTooLarge
    }

    // 4. 读取消息体
    if dataLen > 0 {
        m.Data = make([]byte, dataLen)
        readN, err = io.ReadFull(r, m.Data)
        n += int64(readN)
    } else {
        m.Data = nil
    }

    return n, err
}
```

### 5.6 业务消息结构

```go
// 认证请求
type AuthRequest struct {
    ClientID string `json:"client_id"`
    Token    string `json:"token"`
    Version  string `json:"version"`
}

// 认证响应
type AuthResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

// 隧道配置
type TunnelConfig struct {
    Name       string `json:"name"`
    Type       string `json:"type"`
    LocalAddr  string `json:"local_addr"`
    RemotePort int    `json:"remote_port"`
}

// 隧道注册请求
type RegisterTunnelRequest struct {
    Tunnel TunnelConfig `json:"tunnel"`
}

// 隧道注册响应
type RegisterTunnelResponse struct {
    Success    bool   `json:"success"`
    Message    string `json:"message"`
    TunnelName string `json:"tunnel_name"`
    RemotePort int    `json:"remote_port"`
}

// 新代理请求
type NewProxyRequest struct {
    TunnelName string `json:"tunnel_name"`
    ProxyID    string `json:"proxy_id"`
}

// 代理就绪请求
type ProxyReadyRequest struct {
    ProxyID string `json:"proxy_id"`
}
```

### 5.7 编解码辅助函数

每个业务消息都有对应的编解码函数：

```go
// 编码
func EncodeAuthRequest(req *AuthRequest) ([]byte, error) {
    return json.Marshal(req)
}

// 解码
func DecodeAuthRequest(data []byte) (*AuthRequest, error) {
    req := &AuthRequest{}
    err := json.Unmarshal(data, req)
    return req, err
}
```

**为什么不使用泛型？**
- 保持代码简单明确
- 每个消息类型独立，便于维护
- 编译时类型检查

### 5.8 工具函数

```go
// GetTypeName 返回消息类型的可读名称
func GetTypeName(t uint8) string {
    switch t {
    case TypeAuth:
        return "Auth"
    case TypeAuthResp:
        return "AuthResp"
    // ... 其他类型
    default:
        return "Unknown"
    }
}
```

**用途**：
- 日志输出：`收到消息类型: Auth`
- 错误信息：`期望 AuthResp，收到 Ping`
- 调试定位

## 6. 模块依赖关系

```
                ┌─────────────┐
                │    cmd/     │
                │ main.go     │
                └──────┬──────┘
                       │
        ┌──────────────┼──────────────┐
        │              │              │
        ▼              ▼              ▼
┌───────────┐  ┌───────────┐  ┌───────────┐
│  server   │  │  client   │  │  config   │
└─────┬─────┘  └─────┬─────┘  └───────────┘
      │              │
      └───────┬──────┘
              │
      ┌───────┼───────┐
      │       │       │
      ▼       ▼       ▼
┌─────────┐ ┌───┐ ┌───────┐
│ connect │ │log│ │ proto │
└─────────┘ └───┘ └───────┘
```

**依赖原则**：
- `cmd/` 依赖业务模块
- 业务模块依赖共享模块
- 共享模块之间尽量无依赖
- `proto` 是最底层，无依赖
