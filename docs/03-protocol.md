# 通信协议详解

## 1. 协议概述

Go-Tunnel-Lite 使用自定义的二进制协议进行通信，采用**消息头 + 消息体**的结构设计。

### 1.1 设计原则

| 原则 | 说明 |
|------|------|
| **简单** | 消息结构固定，易于实现和调试 |
| **高效** | 二进制头部，最小化协议开销 |
| **安全** | 长度限制，防止内存耗尽攻击 |
| **可扩展** | 消息体使用 JSON，便于添加字段 |

### 1.2 为什么这样设计？

**二进制消息头的理由：**
- 固定 5 字节，开销极小
- 解析简单快速，无需字符串处理
- 长度字段明确，便于分帧读取

**JSON 消息体的理由：**
- 可读性好，便于调试
- 字段可选，向前兼容
- Go 标准库原生支持

## 2. 消息格式

### 2.1 消息结构

```
+--------+-----------+-------------+
|  Type  |  Length   |    Data     |
| 1 Byte |  4 Bytes  |  N Bytes    |
+--------+-----------+-------------+
   │         │            │
   │         │            └── 消息体（JSON 编码）
   │         └── 消息体长度（大端序 uint32）
   └── 消息类型（uint8）

总长度 = 5 + N 字节
```

### 2.2 字段说明

| 字段 | 类型 | 大小 | 说明 |
|------|------|------|------|
| Type | uint8 | 1 字节 | 消息类型标识 |
| Length | uint32 | 4 字节 | 消息体长度，大端序 |
| Data | []byte | 0-64KB | 消息体，JSON 格式 |

### 2.3 协议常量

```go
const (
    // HeaderLen 消息头长度：Type(1字节) + Length(4字节)
    HeaderLen = 5
    
    // MaxDataLen 最大消息体长度 64KB
    // 防止恶意客户端发送超大消息耗尽内存
    MaxDataLen = 64 * 1024
)
```

**为什么限制 64KB？**
1. 控制消息通常很小（< 1KB），64KB 绰绰有余
2. 数据转发走独立连接，不受此限制
3. 防止恶意构造大消息导致 OOM

## 3. 消息类型

### 3.1 类型分类

消息类型按功能分组，便于管理和扩展：

| 范围 | 分类 | 说明 |
|------|------|------|
| 0x01-0x0F | 认证相关 | 身份验证 |
| 0x10-0x1F | 隧道管理 | 隧道注册与控制 |
| 0x20-0x2F | 代理请求 | 数据通道建立 |
| 0x30-0x3F | 心跳保活 | 连接存活检测 |

### 3.2 类型定义

```go
const (
    // 认证相关 (0x01-0x0F)
    TypeAuth     uint8 = 0x01  // 客户端 → 服务端：认证请求
    TypeAuthResp uint8 = 0x02  // 服务端 → 客户端：认证响应

    // 隧道管理 (0x10-0x1F)
    TypeRegisterTunnel     uint8 = 0x10  // 客户端 → 服务端：注册隧道
    TypeRegisterTunnelResp uint8 = 0x11  // 服务端 → 客户端：注册隧道响应

    // 代理请求 (0x20-0x2F)
    TypeNewProxy   uint8 = 0x20  // 服务端 → 客户端：通知有新连接
    TypeProxyReady uint8 = 0x21  // 客户端 → 服务端：代理准备就绪

    // 心跳保活 (0x30-0x3F)
    TypePing uint8 = 0x30  // 心跳请求
    TypePong uint8 = 0x31  // 心跳响应
)
```

**为什么使用分段编号？**
- 便于识别消息所属分类
- 预留空间便于未来扩展
- 调试时更容易定位问题

## 4. 消息详解

### 4.1 认证请求 (TypeAuth = 0x01)

**方向**：客户端 → 服务端

**作用**：客户端连接后发送的第一条消息，携带认证信息。

**结构**：
```go
type AuthRequest struct {
    ClientID string `json:"client_id"`  // 客户端唯一标识
    Token    string `json:"token"`      // 认证令牌
    Version  string `json:"version"`    // 客户端版本
}
```

**字段说明**：

| 字段 | 必填 | 说明 |
|------|------|------|
| ClientID | 是 | 客户端唯一标识，用于会话管理 |
| Token | 是 | 认证令牌，需与服务端配置一致 |
| Version | 否 | 客户端版本号，用于兼容性检查 |

**示例**：
```json
{
    "client_id": "client-1705123456789",
    "token": "my-secret-token",
    "version": "1.0.0"
}
```

**设计理由**：
- ClientID 自动生成，基于时间戳，保证唯一性
- Token 静态配置，简单有效的认证方式
- Version 预留，便于未来版本兼容处理

---

### 4.2 认证响应 (TypeAuthResp = 0x02)

**方向**：服务端 → 客户端

**作用**：告知客户端认证结果。

**结构**：
```go
type AuthResponse struct {
    Success bool   `json:"success"`  // 是否成功
    Message string `json:"message"`  // 结果消息
}
```

**示例**：
```json
// 成功
{"success": true, "message": "认证成功"}

// 失败
{"success": false, "message": "Token 错误"}
```

**失败原因**：
- Token 错误：配置的令牌不匹配
- 消息格式错误：JSON 解析失败
- 超时：10 秒内未收到认证消息

---

### 4.3 隧道注册请求 (TypeRegisterTunnel = 0x10)

**方向**：客户端 → 服务端

**作用**：认证成功后，注册需要穿透的隧道。

**结构**：
```go
type RegisterTunnelRequest struct {
    Tunnel TunnelConfig `json:"tunnel"`
}

type TunnelConfig struct {
    Name       string `json:"name"`        // 隧道名称
    Type       string `json:"type"`        // 隧道类型（tcp）
    LocalAddr  string `json:"local_addr"`  // 本地服务地址
    RemotePort int    `json:"remote_port"` // 远程暴露端口
}
```

**示例**：
```json
{
    "tunnel": {
        "name": "web",
        "type": "tcp",
        "local_addr": "127.0.0.1:8080",
        "remote_port": 8080
    }
}
```

**字段说明**：

| 字段 | 说明 | 示例 |
|------|------|------|
| name | 隧道标识名，用于日志和管理 | "web", "ssh" |
| type | 隧道类型，目前仅支持 tcp | "tcp" |
| local_addr | 内网服务的地址端口 | "127.0.0.1:8080" |
| remote_port | 服务端暴露的公网端口 | 8080 |

---

### 4.4 隧道注册响应 (TypeRegisterTunnelResp = 0x11)

**方向**：服务端 → 客户端

**作用**：告知隧道注册结果。

**结构**：
```go
type RegisterTunnelResponse struct {
    Success    bool   `json:"success"`
    Message    string `json:"message"`
    TunnelName string `json:"tunnel_name"`
    RemotePort int    `json:"remote_port"`
}
```

**示例**：
```json
// 成功
{
    "success": true,
    "message": "注册成功",
    "tunnel_name": "web",
    "remote_port": 8080
}

// 失败
{
    "success": false,
    "message": "端口不允许使用",
    "remote_port": 0
}
```

**失败原因**：
- 端口不在白名单中
- 端口已被占用
- 请求格式错误

---

### 4.5 新代理请求 (TypeNewProxy = 0x20)

**方向**：服务端 → 客户端

**作用**：当有用户连接公网端口时，通知客户端建立代理通道。

**结构**：
```go
type NewProxyRequest struct {
    TunnelName string `json:"tunnel_name"`  // 所属隧道名称
    ProxyID    string `json:"proxy_id"`     // 代理连接唯一标识
}
```

**示例**：
```json
{
    "tunnel_name": "web",
    "proxy_id": "proxy-1705123456789-abc123"
}
```

**处理流程**：
1. 服务端收到用户连接公网端口
2. 生成唯一 ProxyID
3. 通过控制连接发送 NewProxy 给客户端
4. 等待客户端建立数据连接并回复 ProxyReady

---

### 4.6 代理就绪 (TypeProxyReady = 0x21)

**方向**：客户端 → 服务端

**作用**：客户端完成代理通道建立后，通知服务端可以开始转发数据。

**结构**：
```go
type ProxyReadyRequest struct {
    ProxyID string `json:"proxy_id"`  // 对应的代理连接标识
}
```

**示例**：
```json
{
    "proxy_id": "proxy-1705123456789-abc123"
}
```

**注意**：此消息通过**新建的数据连接**发送，不是控制连接。

---

### 4.7 心跳请求 (TypePing = 0x30)

**方向**：双向（服务端/客户端均可发送）

**作用**：检测连接是否存活。

**结构**：无消息体（Length = 0）

**发送时机**：
- 服务端：每隔 heartbeat_interval 发送
- 客户端：每隔 heartbeat_interval 发送

---

### 4.8 心跳响应 (TypePong = 0x31)

**方向**：接收 Ping 的一方回复

**作用**：证明连接存活。

**结构**：无消息体（Length = 0）

**处理逻辑**：
- 收到 Ping 后立即回复 Pong
- 更新 lastActive 时间戳
- 如果超过 heartbeat_timeout 未收到响应，断开连接

## 5. 编解码实现

### 5.1 消息编码 (WriteTo)

```go
func (m *Message) WriteTo(w io.Writer) (n int64, err error) {
    // 1. 检查消息长度
    dataLen := len(m.Data)
    if dataLen > MaxDataLen {
        return 0, ErrMsgTooLarge
    }

    // 2. 构造消息头（5字节）
    header := make([]byte, HeaderLen)
    header[0] = m.Type                              // 第1字节：消息类型
    binary.BigEndian.PutUint32(header[1:5], uint32(dataLen))  // 第2-5字节：长度

    // 3. 写入消息头
    written, err := w.Write(header)
    n = int64(written)
    if err != nil {
        return n, err
    }

    // 4. 写入消息体（如果有）
    if dataLen > 0 {
        written, err = w.Write(m.Data)
        n += int64(written)
    }

    return n, err
}
```

**关键点**：
- 大端序（Big Endian）：网络字节序，跨平台兼容
- 先检查长度，防止发送超大消息
- 消息体可为空（如 Ping/Pong）

### 5.2 消息解码 (ReadFrom)

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

**关键点**：
- `io.ReadFull` 确保读取完整数据
- 长度检查防止 OOM 攻击
- 空消息体正确处理

### 5.3 JSON 序列化

消息体使用标准 JSON 编解码：

```go
// 编码示例
func EncodeAuthRequest(req *AuthRequest) ([]byte, error) {
    return json.Marshal(req)
}

// 解码示例
func DecodeAuthRequest(data []byte) (*AuthRequest, error) {
    req := &AuthRequest{}
    err := json.Unmarshal(data, req)
    return req, err
}
```

## 6. 协议交互流程

### 6.1 连接建立流程

```
客户端                                服务端
   │                                    │
   │────── TCP Connect ─────────────────►│
   │                                    │
   │────── Auth (token, clientID) ──────►│
   │                                    │ 验证 Token
   │◄───── AuthResp (success) ──────────│
   │                                    │
   │────── RegisterTunnel (tunnel1) ────►│
   │                                    │ 检查端口白名单
   │◄───── RegisterTunnelResp ──────────│
   │                                    │
   │────── RegisterTunnel (tunnel2) ────►│
   │◄───── RegisterTunnelResp ──────────│
   │                                    │
   │        【控制连接建立完成】          │
```

### 6.2 数据代理流程

```
用户                 服务端                客户端              本地服务
 │                    │                    │                    │
 │─── TCP Connect ───►│                    │                    │
 │    (公网端口)       │                    │                    │
 │                    │                    │                    │
 │                    │─── NewProxy ──────►│                    │
 │                    │   (proxyID)        │                    │
 │                    │                    │                    │
 │                    │                    │─── TCP Connect ───►│
 │                    │                    │   (本地服务)        │
 │                    │                    │                    │
 │                    │◄── TCP Connect ────│                    │
 │                    │   (数据连接)        │                    │
 │                    │                    │                    │
 │                    │◄── ProxyReady ─────│                    │
 │                    │   (proxyID)        │                    │
 │                    │                    │                    │
 │──── Request ──────►│════ Forward ═══════│════ Forward ══════►│
 │◄─── Response ──────│════ Forward ═══════│════ Forward ═══════│
 │                    │                    │                    │
```

## 7. 错误处理

### 7.1 协议错误

```go
var (
    ErrMsgTooLarge = errors.New("proto: message too large")
    ErrInvalidMsg  = errors.New("proto: invalid message")
)
```

### 7.2 常见错误场景

| 错误 | 原因 | 处理方式 |
|------|------|----------|
| 消息过大 | 消息体超过 64KB | 返回错误，断开连接 |
| 读取超时 | 网络问题或对方断开 | 关闭连接，清理资源 |
| JSON 解析失败 | 消息体格式错误 | 返回失败响应 |
| 未知消息类型 | 版本不兼容 | 记录日志，忽略消息 |

## 8. 扩展说明

### 8.1 如何添加新消息类型？

1. **定义类型常量**：
```go
const TypeMyNewMsg uint8 = 0x12  // 选择合适的分段
```

2. **定义消息结构**：
```go
type MyNewMsgRequest struct {
    Field1 string `json:"field1"`
    Field2 int    `json:"field2"`
}
```

3. **实现编解码**：
```go
func EncodeMyNewMsgRequest(req *MyNewMsgRequest) ([]byte, error) {
    return json.Marshal(req)
}

func DecodeMyNewMsgRequest(data []byte) (*MyNewMsgRequest, error) {
    req := &MyNewMsgRequest{}
    err := json.Unmarshal(data, req)
    return req, err
}
```

4. **在消息处理中添加分支**：
```go
case proto.TypeMyNewMsg:
    // 处理逻辑
```

### 8.2 版本兼容性

由于消息体使用 JSON：
- **新增字段**：旧版本会忽略新字段
- **删除字段**：新版本使用零值
- **修改字段类型**：需要版本协商（通过 Version 字段）
