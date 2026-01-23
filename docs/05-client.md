# 客户端详解

## 1. 客户端概述

客户端（Client）是 Go-Tunnel-Lite 中运行在内网的组件，负责：
- 连接服务端控制端口
- 发送认证信息
- 注册隧道配置
- 维护心跳保活
- 处理代理请求，建立数据通道
- 在本地服务和服务端之间转发数据

## 2. 核心结构

### 2.1 Client 结构体

```go
type Client struct {
    cfg     *config.ClientConfig  // 客户端配置
    conn    *connect.Connect      // 控制连接
    stopCh  chan struct{}         // 停止信号
    wg      sync.WaitGroup        // 等待所有协程退出
    running bool                  // 运行状态
    mu      sync.Mutex            // 保护 running 状态
}
```

**字段说明**：

| 字段 | 类型 | 作用 | 设计理由 |
|------|------|------|----------|
| cfg | *ClientConfig | 存储配置信息 | 包含服务端地址、隧道列表等 |
| conn | *connect.Connect | 控制连接 | 与服务端的长连接 |
| stopCh | chan struct{} | 优雅关闭 | 通知所有协程退出 |
| wg | sync.WaitGroup | 等待退出 | 确保所有协程完成清理 |
| running | bool | 状态标记 | 防止重复启动/停止 |
| mu | sync.Mutex | 并发保护 | 保护 running 状态 |

## 3. 启动流程

### 3.1 Start 方法

```go
func (c *Client) Start() error {
    c.mu.Lock()
    if c.running {
        c.mu.Unlock()
        return fmt.Errorf("客户端已在运行")
    }
    c.running = true
    c.mu.Unlock()

    // 1. 连接服务端
    if err := c.connect(); err != nil {
        return err
    }

    // 2. 认证
    if err := c.authenticate(); err != nil {
        c.conn.Close()
        return err
    }

    // 3. 注册隧道
    if err := c.registerTunnels(); err != nil {
        c.conn.Close()
        return err
    }

    // 4. 启动消息处理循环
    c.wg.Add(1)
    go c.messageLoop()

    // 5. 启动心跳
    c.wg.Add(1)
    go c.heartbeatLoop()

    log.Info("客户端启动成功")
    return nil
}
```

**启动流程图**：

```
Start()
   │
   ▼
┌─────────────────┐
│  1. connect()   │  ← TCP 连接服务端
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 2.authenticate()│  ← 发送 Token 认证
└────────┬────────┘
         │
         ▼
┌─────────────────────┐
│3.registerTunnels()  │  ← 注册所有隧道
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│ 4. messageLoop()    │  ← 启动消息循环（异步）
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│ 5. heartbeatLoop()  │  ← 启动心跳循环（异步）
└─────────────────────┘
```

**为什么同步执行 1-3 步？**
- 确保连接和认证成功后再启动后台协程
- 错误可以直接返回给调用方
- 失败时便于清理资源

## 4. 连接与认证

### 4.1 connect

```go
func (c *Client) connect() error {
    addr := c.cfg.Client.ServerAddr
    log.Info("正在连接服务端", "addr", addr)

    conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
    if err != nil {
        return fmt.Errorf("连接服务端失败: %w", err)
    }

    c.conn = connect.WrapConnect(conn)
    log.Info("已连接到服务端", "addr", addr)
    return nil
}
```

**设计要点**：

| 要点 | 说明 |
|------|------|
| DialTimeout | 10秒连接超时，避免长时间阻塞 |
| WrapConnect | 封装为消息级连接 |
| 错误包装 | `%w` 保留原始错误信息 |

### 4.2 authenticate

```go
func (c *Client) authenticate() error {
    log.Info("正在进行认证...")

    // 1. 构造认证请求
    authReq := &proto.AuthRequest{
        Token:    c.cfg.Client.Token,
        ClientID: generateClientID(),
        Version:  "1.0.0",
    }

    // 2. 编码并发送
    data, err := proto.EncodeAuthRequest(authReq)
    if err != nil {
        return fmt.Errorf("编码认证请求失败: %w", err)
    }

    msg := &proto.Message{
        Type: proto.TypeAuth,
        Data: data,
    }

    if err := c.conn.WriteMessage(msg); err != nil {
        return fmt.Errorf("发送认证请求失败: %w", err)
    }

    // 3. 读取认证响应
    respMsg, err := c.conn.ReadMessage()
    if err != nil {
        return fmt.Errorf("读取认证响应失败: %w", err)
    }

    if respMsg.Type != proto.TypeAuthResp {
        return fmt.Errorf("期望认证响应，收到: %s", proto.GetTypeName(respMsg.Type))
    }

    // 4. 解码认证响应
    authResp, err := proto.DecodeAuthResponse(respMsg.Data)
    if err != nil {
        return fmt.Errorf("解码认证响应失败: %w", err)
    }

    if !authResp.Success {
        return fmt.Errorf("认证失败: %s", authResp.Message)
    }

    log.Info("认证成功")
    return nil
}
```

**ClientID 生成**：

```go
func generateClientID() string {
    return fmt.Sprintf("client-%d", time.Now().UnixNano())
}
```

**为什么使用时间戳？**
- 简单高效，无需额外依赖
- 纳秒级精度，冲突概率极低
- 便于调试，可以看出连接时间

## 5. 隧道注册

### 5.1 registerTunnels

```go
func (c *Client) registerTunnels() error {
    for _, tunnel := range c.cfg.Client.Tunnels {
        if err := c.registerTunnel(tunnel); err != nil {
            return err
        }
    }
    return nil
}
```

**为什么逐个注册？**
- 每个隧道独立处理，便于错误定位
- 可以获取每个隧道的注册结果
- 支持部分成功（未来可改为警告）

### 5.2 registerTunnel

```go
func (c *Client) registerTunnel(tunnel config.TunnelConfig) error {
    log.Info("正在注册隧道", 
        "name", tunnel.Name, 
        "localAddr", tunnel.LocalAddr, 
        "remotePort", tunnel.RemotePort)

    // 1. 构造注册请求
    req := &proto.RegisterTunnelRequest{
        Tunnel: proto.TunnelConfig{
            Name:       tunnel.Name,
            Type:       "tcp",
            LocalAddr:  tunnel.LocalAddr,
            RemotePort: tunnel.RemotePort,
        },
    }

    // 2. 编码并发送
    data, err := proto.EncodeRegisterTunnelRequest(req)
    if err != nil {
        return fmt.Errorf("编码隧道注册请求失败: %w", err)
    }

    msg := &proto.Message{
        Type: proto.TypeRegisterTunnel,
        Data: data,
    }

    if err := c.conn.WriteMessage(msg); err != nil {
        return fmt.Errorf("发送隧道注册请求失败: %w", err)
    }

    // 3. 读取响应
    respMsg, err := c.conn.ReadMessage()
    if err != nil {
        return fmt.Errorf("读取隧道注册响应失败: %w", err)
    }

    if respMsg.Type != proto.TypeRegisterTunnelResp {
        return fmt.Errorf("期望隧道注册响应，收到: %s", proto.GetTypeName(respMsg.Type))
    }

    // 4. 解码响应
    resp, err := proto.DecodeRegisterTunnelResponse(respMsg.Data)
    if err != nil {
        return fmt.Errorf("解码隧道注册响应失败: %w", err)
    }

    if !resp.Success {
        return fmt.Errorf("注册隧道失败: %s", resp.Message)
    }

    log.Info("隧道注册成功", "name", tunnel.Name, "remotePort", resp.RemotePort)
    return nil
}
```

## 6. 消息循环

### 6.1 messageLoop

```go
func (c *Client) messageLoop() {
    defer c.wg.Done()
    log.Debug("消息处理循环启动")

    for {
        select {
        case <-c.stopCh:
            log.Debug("消息处理循环收到停止信号")
            return
        default:
        }

        // 设置读取超时
        c.conn.SetReadDeadLine(time.Now().Add(60 * time.Second))

        msg, err := c.conn.ReadMessage()
        if err != nil {
            select {
            case <-c.stopCh:
                return
            default:
                if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                    continue  // 超时，继续循环
                }
                log.Error("读取消息失败", "error", err)
                return
            }
        }

        c.handleMessage(msg)
    }
}
```

**设计要点**：

| 要点 | 说明 |
|------|------|
| 60秒读超时 | 心跳间隔的2倍，避免死等 |
| 超时区分 | 超时继续，其他错误退出 |
| 停止信号检查 | 两处检查，确保及时退出 |

### 6.2 handleMessage

```go
func (c *Client) handleMessage(msg *proto.Message) {
    switch msg.Type {
    case proto.TypePing:
        // 收到服务端的 Ping，回复 Pong
        log.Debug("收到服务端心跳，回复Pong")
        pongMsg := &proto.Message{
            Type: proto.TypePong,
            Data: nil,
        }
        if err := c.conn.WriteMessage(pongMsg); err != nil {
            log.Error("回复Pong失败", "error", err)
        }

    case proto.TypePong:
        log.Debug("收到心跳响应")

    case proto.TypeNewProxy:
        // 解码新连接请求
        req, err := proto.DecodeNewProxyRequest(msg.Data)
        if err != nil {
            log.Error("解码新连接请求失败", "error", err)
            return
        }
        log.Info("收到新连接请求", "tunnel", req.TunnelName, "proxyID", req.ProxyID)

        // 异步处理新连接
        go c.handleNewProxy(req)

    default:
        log.Warn("收到未知消息类型", "type", proto.GetTypeName(msg.Type))
    }
}
```

**为什么 NewProxy 异步处理？**
- 不阻塞消息循环
- 可以同时处理多个代理请求
- 单个代理失败不影响其他

## 7. 代理处理

### 7.1 handleNewProxy

```go
func (c *Client) handleNewProxy(req *proto.NewProxyRequest) {
    // 1. 找到对应的隧道配置
    var tunnelCfg *config.TunnelConfig
    for i := range c.cfg.Client.Tunnels {
        if c.cfg.Client.Tunnels[i].Name == req.TunnelName {
            tunnelCfg = &c.cfg.Client.Tunnels[i]
            break
        }
    }

    if tunnelCfg == nil {
        log.Error("找不到隧道配置", "tunnelName", req.TunnelName)
        return
    }

    // 2. 连接本地服务
    localConn, err := net.DialTimeout("tcp", tunnelCfg.LocalAddr, 5*time.Second)
    if err != nil {
        log.Error("连接本地服务失败", "localAddr", tunnelCfg.LocalAddr, "error", err)
        return
    }

    // 3. 建立到服务端的数据连接
    serverConn, err := net.DialTimeout("tcp", c.cfg.Client.ServerAddr, 5*time.Second)
    if err != nil {
        localConn.Close()
        log.Error("建立数据连接失败", "error", err)
        return
    }

    dataConn := connect.WrapConnect(serverConn)

    // 4. 发送 ProxyReady 消息
    readyReq := &proto.ProxyReadyRequest{
        ProxyID: req.ProxyID,
    }
    data, _ := proto.EncodeProxyReadyRequest(readyReq)
    readyMsg := &proto.Message{
        Type: proto.TypeProxyReady,
        Data: data,
    }

    if err := dataConn.WriteMessage(readyMsg); err != nil {
        localConn.Close()
        dataConn.Close()
        log.Error("发送 ProxyReady 失败", "error", err)
        return
    }

    log.Info("数据通道建立成功", "proxyID", req.ProxyID)

    // 5. 开始双向转发数据
    go c.proxyData(localConn, dataConn.RawConn(), req.ProxyID)
}
```

**代理建立流程图**：

```
handleNewProxy()
       │
       ▼
┌──────────────────┐
│ 1. 查找隧道配置   │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ 2. 连接本地服务   │  ← net.Dial(127.0.0.1:8080)
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ 3. 建立数据连接   │  ← 新建到服务端的 TCP 连接
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ 4. 发送ProxyReady│  ← 通知服务端可以转发
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ 5. proxyData()   │  ← 启动双向数据转发
└──────────────────┘
```

### 7.2 proxyData

```go
func (c *Client) proxyData(local net.Conn, remote net.Conn, proxyID string) {
    defer local.Close()
    defer remote.Close()

    // 使用 WaitGroup 等待两个方向的转发都完成
    var wg sync.WaitGroup
    wg.Add(2)

    // local -> remote
    go func() {
        defer wg.Done()
        n, _ := io.Copy(remote, local)
        log.Debug("转发完成", "proxyID", proxyID, "direction", "local->remote", "bytes", n)
    }()

    // remote -> local
    go func() {
        defer wg.Done()
        n, _ := io.Copy(local, remote)
        log.Debug("转发完成", "proxyID", proxyID, "direction", "remote->local", "bytes", n)
    }()

    wg.Wait()
    log.Info("代理连接关闭", "proxyID", proxyID)
}
```

**双向转发示意**：

```
        ┌────────────────────────────────────────┐
        │                客户端                   │
        │                                        │
用户 ←──│←── remote ←── io.Copy ←── local ←──   │←── 本地服务
        │                                        │
用户 ──►│──► remote ──► io.Copy ──► local ───►  │──► 本地服务
        │                                        │
        └────────────────────────────────────────┘
```

**为什么使用 io.Copy？**
- 高效：使用内核级别的数据拷贝
- 简洁：一行代码完成转发
- 自动处理 EOF：一端关闭时自动结束

**为什么两个方向各一个协程？**
- TCP 是全双工，两个方向独立
- 单向关闭不影响另一方向
- 最大化吞吐量

## 8. 心跳循环

### 8.1 heartbeatLoop

```go
func (c *Client) heartbeatLoop() {
    defer c.wg.Done()

    interval := time.Duration(c.cfg.Client.HeartbeatInterval) * time.Second
    if interval <= 0 {
        interval = 30 * time.Second
    }

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    log.Debug("心跳循环启动", "interval", interval)

    for {
        select {
        case <-c.stopCh:
            log.Debug("心跳循环收到停止信号")
            return
        case <-ticker.C:
            if err := c.sendHeartbeat(); err != nil {
                log.Error("发送心跳失败", "error", err)
                return
            }
        }
    }
}
```

### 8.2 sendHeartbeat

```go
func (c *Client) sendHeartbeat() error {
    msg := &proto.Message{
        Type: proto.TypePing,
        Data: nil,
    }
    return c.conn.WriteMessage(msg)
}
```

**心跳时序**：

```
时间  0s      30s     60s     90s
      │       │       │       │
客户端├─Ping──┼─Ping──┼─Ping──┤
      │       │       │       │
服务端├─Pong──┼─Pong──┼─Pong──┤

同时，服务端也在发送 Ping，客户端回复 Pong
形成双向心跳检测
```

## 9. 停止流程

### 9.1 Stop 方法

```go
func (c *Client) Stop() {
    c.mu.Lock()
    if !c.running {
        c.mu.Unlock()
        return
    }
    c.running = false
    c.mu.Unlock()

    // 发送停止信号
    close(c.stopCh)

    // 关闭控制连接
    if c.conn != nil {
        c.conn.Close()
    }

    // 等待所有协程退出
    c.wg.Wait()
    log.Info("客户端已停止")
}
```

**停止顺序**：

1. **标记停止状态**：防止重复调用
2. **发送停止信号**：通知所有协程
3. **关闭连接**：触发读取错误，退出循环
4. **等待协程**：确保资源释放

## 10. 入口程序

### 10.1 main.go

```go
func main() {
    flag.Parse()

    if *showHelp {
        printUsage()
        os.Exit(0)
    }

    // 1. 加载配置
    cfg, err := config.LoadClientConfig(*configFile)
    if err != nil {
        fmt.Printf("加载配置失败: %v\n", err)
        os.Exit(1)
    }

    // 2. 创建客户端
    cli := client.NewClient(cfg)

    // 3. 启动客户端
    if err := cli.Start(); err != nil {
        log.Error("客户端启动失败", "error", err)
        os.Exit(1)
    }

    log.Info("客户端启动成功!")
    log.Info("已连接服务端", "addr", cfg.Client.ServerAddr)
    log.Info("注册隧道数量", "count", len(cfg.Client.Tunnels))
    
    for _, t := range cfg.Client.Tunnels {
        log.Info("隧道", "name", t.Name, "remote_port", t.RemotePort, "local_addr", t.LocalAddr)
    }

    // 4. 等待退出信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    sig := <-sigCh

    log.Info("收到信号，正在关闭客户端...", "signal", sig)

    // 5. 优雅关闭
    cli.Stop()

    log.Info("客户端已关闭")
}
```

## 11. 命令行参数

```
用法:
  go-tunnel-client [选项]

选项:
  -c string    配置文件路径 (默认 "client.yaml")
  -h           显示帮助信息

示例:
  go-tunnel-client -c /etc/tunnel/client.yaml
```

## 12. 错误处理策略

| 错误类型 | 处理方式 |
|----------|----------|
| 连接失败 | 启动失败，返回错误 |
| 认证失败 | 启动失败，返回错误 |
| 隧道注册失败 | 启动失败，返回错误 |
| 心跳发送失败 | 退出心跳循环 |
| 消息读取失败 | 退出消息循环 |
| 本地服务连接失败 | 记录日志，放弃该代理 |
| 数据转发错误 | 关闭代理连接 |
