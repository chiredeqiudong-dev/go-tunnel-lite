# 服务端详解

## 1. 服务端概述

服务端（Server）是 Go-Tunnel-Lite 的核心组件，负责：
- 监听控制端口，接受客户端连接
- 验证客户端身份（Token 认证）
- 管理客户端会话
- 处理隧道注册请求
- 维护心跳检测

## 2. 核心结构

### 2.1 Server 结构体

```go
type Server struct {
    cfg *config.ServerConfig      // 服务端配置

    listener   net.Listener       // TCP 监听器
    sessions   map[string]*ClientSession  // 客户端会话映射（key: clientID）
    sessionsMu sync.RWMutex       // 会话映射的读写锁

    stopCh chan struct{}          // 停止信号通道
    wg     sync.WaitGroup         // 等待所有协程退出
}
```

**字段说明**：

| 字段 | 类型 | 作用 | 设计理由 |
|------|------|------|----------|
| cfg | *ServerConfig | 存储配置信息 | 避免重复读取配置文件 |
| listener | net.Listener | TCP 监听器 | 接受新连接 |
| sessions | map[string]*ClientSession | 会话管理 | 通过 clientID 快速查找会话 |
| sessionsMu | sync.RWMutex | 并发保护 | 读写锁，支持多读单写 |
| stopCh | chan struct{} | 优雅关闭 | 通知所有协程退出 |
| wg | sync.WaitGroup | 等待退出 | 确保所有协程完成清理 |

### 2.2 ClientSession 结构体

```go
type ClientSession struct {
    clientID   string           // 客户端唯一标识
    conn       *connect.Connect // 控制连接
    lastActive time.Time        // 最后活跃时间

    stopCh chan struct{}        // 会话停止信号
    mu     sync.Mutex           // 保护 lastActive
}
```

**字段说明**：

| 字段 | 作用 | 设计理由 |
|------|------|----------|
| clientID | 标识客户端 | 日志追踪、会话管理 |
| conn | 封装的 TCP 连接 | 提供消息级读写 |
| lastActive | 心跳检测 | 判断是否超时 |
| stopCh | 单会话停止 | 关闭单个客户端不影响其他 |
| mu | 并发保护 | lastActive 被多协程访问 |

## 3. 生命周期管理

### 3.1 启动流程

```
NewServer() → Start() → acceptLoop()
                │
                └─► handleNewConnection() [per connection]
                            │
                            ├─► 认证验证
                            ├─► 创建会话
                            └─► handleSession()
                                    │
                                    ├─► heartbeatLoop()
                                    └─► 消息循环
```

#### 3.1.1 NewServer

```go
func NewServer(cfg *config.ServerConfig) *Server {
    return &Server{
        cfg:      cfg,
        sessions: make(map[string]*ClientSession),
        stopCh:   make(chan struct{}),
    }
}
```

**作用**：创建服务端实例，初始化数据结构。

**为什么不在这里启动监听？**
- 分离创建和启动，便于测试
- 允许在启动前进行额外配置
- 错误处理更清晰

#### 3.1.2 Start

```go
func (s *Server) Start() error {
    // 1. 监听控制端口
    listener, err := net.Listen("tcp", s.cfg.Server.ControlAddr)
    if err != nil {
        return err
    }
    s.listener = listener
    
    log.Info("服务端启动，监听控制端口", "addr", s.cfg.Server.ControlAddr)

    // 2. 启动接受连接的协程
    s.wg.Add(1)
    go s.acceptLoop()

    return nil
}
```

**作用**：启动服务端，开始监听。

**关键点**：
- 同步执行监听，失败立即返回错误
- 异步启动 acceptLoop，不阻塞调用方
- 使用 WaitGroup 跟踪协程

### 3.2 停止流程

```go
func (s *Server) Stop() {
    log.Info("正在停止服务端...")

    // 1. 发送停止信号
    close(s.stopCh)

    // 2. 关闭监听器（使 Accept 返回错误）
    if s.listener != nil {
        s.listener.Close()
    }

    // 3. 关闭所有客户端会话
    s.sessionsMu.Lock()
    for _, session := range s.sessions {
        session.Close()
    }
    s.sessionsMu.Unlock()

    // 4. 等待所有协程退出
    s.wg.Wait()

    log.Info("服务端已停止")
}
```

**优雅关闭的设计理由**：

1. **stopCh 信号**：通知所有协程准备退出
2. **关闭监听器**：Accept 调用会返回错误，退出循环
3. **关闭会话**：确保所有客户端断开连接
4. **WaitGroup 等待**：确保资源完全释放

## 4. 连接处理

### 4.1 acceptLoop

```go
func (s *Server) acceptLoop() {
    defer s.wg.Done()

    for {
        conn, err := s.listener.Accept()
        if err != nil {
            select {
            case <-s.stopCh:
                return  // 正常退出
            default:
                log.Error("接受连接失败", "error", err)
                continue  // 临时错误，继续接受
            }
        }

        // 启动协程处理新连接
        s.wg.Add(1)
        go s.handleNewConnection(conn)
    }
}
```

**设计要点**：

| 要点 | 说明 |
|------|------|
| 无限循环 | 持续接受新连接 |
| 错误区分 | 正常关闭 vs 临时错误 |
| 每连接一协程 | 并发处理，互不阻塞 |
| WaitGroup 跟踪 | 确保优雅关闭 |

### 4.2 handleNewConnection

```go
func (s *Server) handleNewConnection(rawConn net.Conn) {
    defer s.wg.Done()

    conn := connect.WrapConnect(rawConn)
    remoteAddr := conn.RemoteAddr().String()
    log.Info("新连接", "remoteAddr", remoteAddr)

    // 1. 设置认证超时（10秒）
    conn.SetDeadline(time.Now().Add(10 * time.Second))

    // 2. 等待认证消息
    msg, err := conn.ReadMessage()
    if err != nil {
        log.Warn("读取认证消息失败", "remoteAddr", remoteAddr, "error", err)
        conn.Close()
        return
    }

    // 3. 验证消息类型
    if msg.Type != proto.TypeAuth {
        log.Warn("期望认证消息，收到", "type", msg.Type)
        conn.Close()
        return
    }

    // 4. 解析认证信息
    authReq, err := proto.DecodeAuthRequest(msg.Data)
    if err != nil {
        log.Warn("解析认证消息失败", "error", err)
        s.sendAuthResponse(conn, false, "认证消息格式错误")
        conn.Close()
        return
    }

    // 5. 验证 Token
    if authReq.Token != s.cfg.Server.Token {
        log.Warn("Token 验证失败", "clientID", authReq.ClientID)
        s.sendAuthResponse(conn, false, "Token 错误")
        conn.Close()
        return
    }

    // 6. 处理重复连接
    s.sessionsMu.Lock()
    if oldSession, exists := s.sessions[authReq.ClientID]; exists {
        log.Warn("客户端重复连接，关闭旧连接", "clientID", authReq.ClientID)
        oldSession.Close()
        delete(s.sessions, authReq.ClientID)
    }
    s.sessionsMu.Unlock()

    // 7. 清除超时设置
    conn.SetDeadline(time.Time{})

    // 8. 发送认证成功响应
    s.sendAuthResponse(conn, true, "认证成功")
    log.Info("客户端认证成功", "clientID", authReq.ClientID)

    // 9. 创建并注册会话
    session := &ClientSession{
        clientID:   authReq.ClientID,
        conn:       conn,
        lastActive: time.Now(),
        stopCh:     make(chan struct{}),
    }

    s.sessionsMu.Lock()
    s.sessions[authReq.ClientID] = session
    s.sessionsMu.Unlock()

    // 10. 处理会话
    s.handleSession(session)

    // 11. 会话结束，清理
    s.sessionsMu.Lock()
    if s.sessions[authReq.ClientID] == session {
        delete(s.sessions, authReq.ClientID)
    }
    s.sessionsMu.Unlock()
    log.Info("客户端断开", "clientID", authReq.ClientID)
}
```

**认证流程设计理由**：

| 步骤 | 理由 |
|------|------|
| 10秒超时 | 防止连接占用不认证 |
| 检查消息类型 | 第一条必须是认证消息 |
| 解析失败返回错误 | 给客户端明确反馈 |
| Token 验证 | 简单有效的身份验证 |
| 关闭旧连接 | 防止资源泄漏，支持重连 |
| 清除超时 | 认证后切换到心跳超时 |

## 5. 会话处理

### 5.1 handleSession

```go
func (s *Server) handleSession(session *ClientSession) {
    // 启动心跳检测
    go s.heartbeatLoop(session)

    // 消息循环
    for {
        select {
        case <-s.stopCh:
            return
        case <-session.stopCh:
            return
        default:
        }

        // 设置读取超时
        session.conn.SetReadDeadLine(
            time.Now().Add(s.cfg.Server.HeartbeatTimeout + 5*time.Second))

        msg, err := session.conn.ReadMessage()
        if err != nil {
            if session.IsClosed() {
                return
            }
            log.Warn("读取消息失败", "clientID", session.clientID, "error", err)
            session.Close()
            return
        }

        // 更新活跃时间
        session.mu.Lock()
        session.lastActive = time.Now()
        session.mu.Unlock()

        // 处理消息
        s.handleMessage(session, msg)
    }
}
```

**消息循环设计要点**：

| 要点 | 说明 |
|------|------|
| 双重退出检查 | 服务停止 或 会话关闭 |
| 读取超时 | 比心跳超时稍长，防止误判 |
| 更新活跃时间 | 收到任何消息都算活跃 |
| 错误处理 | 读取失败则关闭会话 |

### 5.2 handleMessage

```go
func (s *Server) handleMessage(session *ClientSession, msg *proto.Message) {
    switch msg.Type {
    case proto.TypePing:
        // 响应心跳
        pong := &proto.Message{Type: proto.TypePong}
        if err := session.conn.WriteMessage(pong); err != nil {
            log.Warn("发送 Pong 失败", "clientID", session.clientID, "error", err)
        }

    case proto.TypePong:
        // 收到 Pong，更新活跃时间（已在上面更新）
        log.Debug("收到 Pong", "clientID", session.clientID)

    case proto.TypeRegisterTunnel:
        // 处理隧道注册请求
        s.handleRegisterTunnel(session, msg)

    default:
        log.Warn("未知消息类型", "type", msg.Type, "clientID", session.clientID)
    }
}
```

**为什么使用 switch？**
- 消息类型是枚举值，switch 最清晰
- 便于添加新消息类型
- 编译器可检查遗漏的 case

## 6. 隧道注册

### 6.1 handleRegisterTunnel

```go
func (s *Server) handleRegisterTunnel(session *ClientSession, msg *proto.Message) {
    // 1. 解码请求
    req, err := proto.DecodeRegisterTunnelRequest(msg.Data)
    if err != nil {
        log.Error("解码隧道注册请求失败", "error", err)
        s.sendRegisterTunnelResponse(session, false, "请求格式错误", 0)
        return
    }

    log.Info("收到隧道注册请求", 
        "clientID", session.clientID, 
        "tunnelName", req.Tunnel.Name, 
        "remotePort", req.Tunnel.RemotePort)

    // 2. 验证端口白名单
    if !s.isPortAllowed(req.Tunnel.RemotePort) {
        log.Warn("端口不在白名单中", "remotePort", req.Tunnel.RemotePort)
        s.sendRegisterTunnelResponse(session, false, "端口不允许使用", 0)
        return
    }

    // 3. 返回成功（TODO: 实际启动端口监听）
    s.sendRegisterTunnelResponse(session, true, "注册成功", req.Tunnel.RemotePort)
    log.Info("隧道注册成功", "tunnelName", req.Tunnel.Name)
}
```

### 6.2 端口白名单检查

```go
func (s *Server) isPortAllowed(port int) bool {
    publicPorts := s.cfg.Server.PublicPorts
    
    // 白名单为空，允许所有端口
    if len(publicPorts) == 0 {
        return true
    }
    
    // 检查端口是否在白名单中
    for _, allowedPort := range publicPorts {
        if port == allowedPort {
            return true
        }
    }
    return false
}
```

**白名单设计理由**：

| 场景 | 配置 | 行为 |
|------|------|------|
| 开发环境 | 空白名单 | 允许所有端口 |
| 生产环境 | 指定端口列表 | 仅允许列表内端口 |

**安全考虑**：
- 防止客户端占用系统端口（如 22、80）
- 限制端口范围，便于防火墙配置
- 避免端口冲突

## 7. 心跳检测

### 7.1 heartbeatLoop

```go
func (s *Server) heartbeatLoop(session *ClientSession) {
    ticker := time.NewTicker(s.cfg.Server.HeartbeatInterval)
    defer ticker.Stop()

    for {
        select {
        case <-s.stopCh:
            return
        case <-session.stopCh:
            return
        case <-ticker.C:
            // 检查是否超时
            session.mu.Lock()
            lastActive := session.lastActive
            session.mu.Unlock()

            if time.Since(lastActive) > s.cfg.Server.HeartbeatTimeout {
                log.Warn("客户端心跳超时", "clientID", session.clientID)
                session.Close()
                return
            }

            // 发送 Ping
            ping := &proto.Message{Type: proto.TypePing}
            if err := session.conn.WriteMessage(ping); err != nil {
                log.Warn("发送 Ping 失败", "clientID", session.clientID, "error", err)
                session.Close()
                return
            }
        }
    }
}
```

**心跳机制设计**：

```
时间线:
0s          30s         60s         90s        120s
│           │           │           │           │
└─ Ping ────┴─ Ping ────┴─ Ping ────┴─ 超时！ ──┘
            │           │
            └─ Pong ────┘  (正常)
            
配置:
- heartbeat_interval: 30s (发送间隔)
- heartbeat_timeout: 90s (超时阈值)
```

**为什么 timeout > interval × 2？**
- 允许偶尔丢失一次心跳
- 网络抖动不会立即断开
- 给足够时间恢复

## 8. 会话管理

### 8.1 Close 方法

```go
func (cs *ClientSession) Close() {
    cs.mu.Lock()
    defer cs.mu.Unlock()

    select {
    case <-cs.stopCh:
        return // 已关闭
    default:
        close(cs.stopCh)
    }

    cs.conn.Close()
}
```

**为什么检查是否已关闭？**
- 防止重复 close channel（会 panic）
- 幂等性：多次调用 Close 不会出错
- 并发安全：可能被多处调用

### 8.2 IsClosed 方法

```go
func (cs *ClientSession) IsClosed() bool {
    select {
    case <-cs.stopCh:
        return true
    default:
        return false
    }
}
```

**为什么使用 select + default？**
- 非阻塞检查 channel 是否关闭
- 关闭的 channel 读取立即返回
- 未关闭的 channel 走 default

## 9. 入口程序

### 9.1 main.go

```go
func main() {
    flag.Parse()

    if *showHelp {
        printUsage()
        os.Exit(0)
    }

    // 1. 加载配置
    cfg, err := config.LoadServerConfig(*configFile)
    if err != nil {
        fmt.Printf("加载配置失败: %v\n", err)
        os.Exit(1)
    }

    // 2. 创建服务端
    srv := server.NewServer(cfg)

    // 3. 启动服务
    if err := srv.Start(); err != nil {
        log.Error("服务端启动失败", "error", err)
        os.Exit(1)
    }

    log.Info("服务端启动成功!")
    log.Info("控制端口", "addr", cfg.Server.ControlAddr)

    // 4. 等待退出信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    sig := <-sigCh

    log.Info("收到信号，正在关闭服务...", "signal", sig)

    // 5. 优雅关闭
    srv.Stop()

    log.Info("服务端已关闭")
}
```

**信号处理设计**：

| 信号 | 来源 | 行为 |
|------|------|------|
| SIGINT | Ctrl+C | 优雅关闭 |
| SIGTERM | kill 命令 | 优雅关闭 |

## 10. 命令行参数

```
用法:
  go-tunnel-server [选项]

选项:
  -c string    配置文件路径 (默认 "server.yaml")
  -h           显示帮助信息

示例:
  go-tunnel-server -c /etc/tunnel/server.yaml
```
