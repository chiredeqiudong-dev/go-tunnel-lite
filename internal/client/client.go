package client

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/config"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/connect"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

// Client 客户端
type Client struct {
	cfg         *config.ClientConfig
	conn        *connect.Connect                // 控制连接
	stopCh      chan struct{}                   // 停止信号
	wg          sync.WaitGroup                  // 等待所有协程退出
	running     bool                            // 运行状态
	mu          sync.Mutex                      // 保护 running 状态
	tunnelCache map[string]*config.TunnelConfig // 隧道配置缓存
	processor   *BatchProcessor                 // 消息批量处理器
}

// NewClient 创建客户端
func NewClient(cfg *config.ClientConfig) *Client {
	client := &Client{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}

	// 初始化批量处理器
	client.processor = NewBatchProcessor(2, 10, client.handleBatchMessages)

	return client
}

// Start 启动客户端
func (c *Client) Start() error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("客户端已在运行")
	}
	c.running = true
	c.mu.Unlock()

	// 初始化隧道配置缓存
	c.tunnelCache = make(map[string]*config.TunnelConfig)
	for i := range c.cfg.Client.Tunnels {
		c.tunnelCache[c.cfg.Client.Tunnels[i].Name] = &c.cfg.Client.Tunnels[i]
	}

	// 连接服务端
	if err := c.connect(); err != nil {
		return err
	}

	// 认证
	if err := c.authenticate(); err != nil {
		c.conn.Close()
		return err
	}

	// 注册隧道
	if err := c.registerTunnels(); err != nil {
		c.conn.Close()
		return err
	}

	// 启动批量处理器
	c.processor.Start()

	// 启动消息处理循环
	c.wg.Add(1)
	go c.messageLoop()

	// 启动心跳
	c.wg.Add(1)
	go c.heartbeatLoop()

	log.Info("客户端启动成功")
	return nil
}

// Stop 停止客户端
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

	// 停止批量处理器
	c.processor.Stop()

	// 等待所有协程退出
	c.wg.Wait()
	log.Info("客户端已停止")
}

// connect 连接服务端
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

// authenticate 认证
func (c *Client) authenticate() error {
	log.Info("正在进行认证...")

	// 构造认证请求
	authReq := &proto.AuthRequest{
		Token:    c.cfg.Client.Token,
		ClientID: fmt.Sprintf("client-%d", time.Now().UnixNano()),
		Version:  "1.0.0", // 用处？
	}

	// 编码并发送
	data, err := proto.Encode(authReq)
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

	// 读取认证响应
	respMsg, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取认证响应失败: %w", err)
	}
	if respMsg.Type != proto.TypeAuthResp {
		return fmt.Errorf("期望认证响应，收到: %s", proto.GetTypeName(respMsg.Type))
	}

	// 解码认证响应
	authResp, err := proto.Decode[proto.AuthResponse](respMsg.Data)
	if err != nil {
		return fmt.Errorf("解码认证响应失败: %w", err)
	}
	if !authResp.Success {
		return fmt.Errorf("认证失败: %s", authResp.Message)
	}

	log.Info("认证成功")
	return nil
}

// registerTunnels 注册所有隧道
func (c *Client) registerTunnels() error {
	for _, tunnel := range c.cfg.Client.Tunnels {
		if err := c.registerTunnel(tunnel); err != nil {
			return err
		}
	}
	return nil
}

// registerTunnel 注册单个隧道
func (c *Client) registerTunnel(tunnel config.TunnelConfig) error {
	log.Info("正在注册隧道", "name", tunnel.Name, "localAddr", tunnel.LocalAddr, "remotePort", tunnel.RemotePort)

	// 构造注册请求
	req := &proto.RegisterTunnelRequest{
		Tunnel: proto.TunnelConfig{
			Name:       tunnel.Name,
			Type:       "tcp", // 默认 tcp 类型
			LocalAddr:  tunnel.LocalAddr,
			RemotePort: tunnel.RemotePort,
		},
	}

	// 编码并发送
	data, err := proto.Encode(req)
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

	// 读取响应
	respMsg, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取隧道注册响应失败: %w", err)
	}

	if respMsg.Type != proto.TypeRegisterTunnelResp {
		return fmt.Errorf("期望隧道注册响应，收到: %s", proto.GetTypeName(respMsg.Type))
	}

	// 解码响应
	resp, err := proto.Decode[proto.RegisterTunnelResponse](respMsg.Data)
	if err != nil {
		return fmt.Errorf("解码隧道注册响应失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("注册隧道失败: %s", resp.Message)
	}

	log.Info("隧道注册成功", "name", tunnel.Name, "remotePort", resp.RemotePort)
	return nil
}

// messageLoop 消息处理循环
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
					continue // 超时，继续循环
				}
				log.Error("读取消息失败", "error", err)
				return
			}
		}

		// 将消息推送到批量处理器
		c.processor.Push(msg)
	}
}

// handleBatchMessages 批量处理消息
func (c *Client) handleBatchMessages(messages []*proto.Message) {
	for _, msg := range messages {
		c.handleSingleMessage(msg)
	}
}

// handleSingleMessage 处理单条消息
func (c *Client) handleSingleMessage(msg *proto.Message) {
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
		req, err := proto.Decode[proto.NewProxyRequest](msg.Data)
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

// handleNewProxy 处理新代理连接请求
func (c *Client) handleNewProxy(req *proto.NewProxyRequest) {
	// 1. 从缓存中查找对应的隧道配置
	tunnelCfg, exists := c.tunnelCache[req.TunnelName]
	if !exists {
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
	data, _ := proto.Encode(readyReq)
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

// proxyData 双向转发数据
func (c *Client) proxyData(local net.Conn, remote net.Conn, proxyID string) {
	defer local.Close()
	defer remote.Close()

	// 使用 WaitGroup 等待两个方向的转发都完成
	var wg sync.WaitGroup
	wg.Add(2)

	// local -> remote (使用 io.Copy 实现零拷贝)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(remote, local)
		log.Debug("转发完成", "proxyID", proxyID, "direction", "local->remote", "bytes", n)
	}()

	// remote -> local (使用 io.Copy 实现零拷贝)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(local, remote)
		log.Debug("转发完成", "proxyID", proxyID, "direction", "remote->local", "bytes", n)
	}()

	wg.Wait()
	log.Info("代理连接关闭", "proxyID", proxyID)
}

// heartbeatLoop 心跳循环
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

// sendHeartbeat 发送心跳
func (c *Client) sendHeartbeat() error {
	msg := &proto.Message{
		Type: proto.TypePing,
		Data: nil,
	}
	return c.conn.WriteMessage(msg)
}
