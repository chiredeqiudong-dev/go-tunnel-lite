package server

import (
	"net"
	"sync"
	"time"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/config"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/connect"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

/*
服务端核心
1. 监听控制端口，接受客户端连接
2. 处理客户端认证
3. 管理客户端会话
4. 心跳检测
*/

// 服务端
type Server struct {
	cfg *config.ServerConfig

	listener   net.Listener
	sessions   map[string]*ClientSession
	sessionsMu sync.RWMutex

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// 客户端会话
type ClientSession struct {
	clientID   string
	conn       *connect.Connect
	lastActive time.Time

	stopCh chan struct{}
	mu     sync.Mutex
}

// 创建服务端实例
func NewServer(cfg *config.ServerConfig) *Server {
	return &Server{
		cfg:      cfg,
		sessions: make(map[string]*ClientSession),
		stopCh:   make(chan struct{}),
	}
}

// 启动服务端
func (s *Server) Start() error {
	// 监听控制端口
	listener, err := net.Listen("tcp", s.cfg.Server.ControlAddr)
	if err != nil {
		return err
	}

	s.listener = listener
	log.Info("服务端启动，监听控制端口", "addr", s.cfg.Server.ControlAddr)

	// 启动接受连接的协程
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop 服务端
func (s *Server) Stop() {
	log.Info("正在停止服务端...")

	// 发送停止信号
	close(s.stopCh)

	// 关闭鉴听器
	if s.listener != nil {
		s.listener.Close()
	}

	// 关闭所有客户端会话
	s.sessionsMu.Lock()
	for _, session := range s.sessions {
		session.Close()
	}
	s.sessionsMu.Unlock()

	// 等待所有协程退出
	s.wg.Wait()

	log.Info("服务端已停止")
}

// 接受客户端连接
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		connect, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				log.Error("接受连接失败", "error", err)
				continue
			}
		}

		// 启动协程处理新连接
		s.wg.Add(1)
		go s.handleNewConnection(connect)

	}
}

// 处理新的客户端连接
func (s *Server) handleNewConnection(rawConn net.Conn) {
	defer s.wg.Done()

	connect := connect.WrapConnect(rawConn)
	remoteAddr := connect.RemoteAddr().String()
	log.Info("新连接", "remoteAddr", remoteAddr)

	// 设置认证超时
	connect.SetDeadline(time.Now().Add(10 * time.Second))

	// 等待认证消息
	msg, err := connect.ReadMessage()
	if err != nil {
		log.Warn("读取认证消息失败", "remoteAddr", remoteAddr, "error", err)
		connect.Close()
		return
	}

	// 验证消息类型
	if msg.Type != proto.TypeAuth {
		log.Warn("期望认证消息，收到", "type", msg.Type, "remoteAddr", remoteAddr)
		connect.Close()
		return
	}

	// 解析认证信息
	authReq, err := proto.Decode[proto.AuthRequest](msg.Data)
	if err != nil {
		log.Warn("解析认证消息失败", "remoteAddr", remoteAddr, "error", err)
		s.sendAuthResponse(connect, false, "认证消息格式错误")
		connect.Close()
		return
	}

	// 验证 Token
	if authReq.Token != s.cfg.Server.Token {
		log.Warn("Token 验证失败", "remoteAddr", remoteAddr, "clientID", authReq.ClientID)
		s.sendAuthResponse(connect, false, "Token 错误")
		connect.Close()
		return
	}

	// 检查是否已存在相同 clientID 的会话
	s.sessionsMu.Lock()
	if oldSession, exists := s.sessions[authReq.ClientID]; exists {
		log.Warn("客户端重复连接，关闭旧连接", "clientID", authReq.ClientID)
		oldSession.Close()
		delete(s.sessions, authReq.ClientID)
	}
	s.sessionsMu.Unlock()

	// 清除超时设置
	connect.SetDeadline(time.Time{})

	// 发送认证成功响应
	s.sendAuthResponse(connect, true, "认证成功")
	log.Info("客户端认证成功", "clientID", authReq.ClientID, "remoteAddr", remoteAddr)

	// 创建会话
	session := &ClientSession{
		clientID:   authReq.ClientID,
		conn:       connect,
		lastActive: time.Now(),
		stopCh:     make(chan struct{}),
	}

	// 注册会话
	s.sessionsMu.Lock()
	s.sessions[authReq.ClientID] = session
	s.sessionsMu.Unlock()

	// 处理会话
	s.handleSession(session)

	// 会话结束，清理（只有当前会话是自己时才删除）
	s.sessionsMu.Lock()
	if s.sessions[authReq.ClientID] == session {
		delete(s.sessions, authReq.ClientID)
	}
	s.sessionsMu.Unlock()
	log.Info("客户端断开", "clientID", authReq.ClientID)
}

// 处理客户端会话（消息循环）
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

		// 设置读取超时（比心跳超时稍长）
		session.conn.SetReadDeadLine(time.Now().Add(s.cfg.Server.HeartbeatTimeout + 5*time.Second))

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

// 处理单条消息
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

// handleRegisterTunnel 处理隧道注册请求
func (s *Server) handleRegisterTunnel(session *ClientSession, msg *proto.Message) {
	// 解码请求
	req, err := proto.Decode[proto.RegisterTunnelRequest](msg.Data)
	if err != nil {
		log.Error("解码隧道注册请求失败", "clientID", session.clientID, "error", err)
		s.sendRegisterTunnelResponse(session, false, "请求格式错误", 0)
		return
	}

	log.Info("收到隧道注册请求", "clientID", session.clientID, "tunnelName", req.Tunnel.Name, "remotePort", req.Tunnel.RemotePort)

	// 验证端口是否在白名单中
	if !s.isPortAllowed(req.Tunnel.RemotePort) {
		log.Warn("端口不在白名单中", "clientID", session.clientID, "remotePort", req.Tunnel.RemotePort)
		s.sendRegisterTunnelResponse(session, false, "端口不允许使用", 0)
		return
	}

	// TODO: 实际启动代理监听端口的逻辑
	// 这里暂时直接返回成功

	s.sendRegisterTunnelResponse(session, true, "注册成功", req.Tunnel.RemotePort)
	log.Info("隧道注册成功", "clientID", session.clientID, "tunnelName", req.Tunnel.Name, "remotePort", req.Tunnel.RemotePort)
}

// isPortAllowed 检查端口是否在白名单中
// 如果 public_ports 为空，则允许所有端口
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

// sendRegisterTunnelResponse 发送隧道注册响应
func (s *Server) sendRegisterTunnelResponse(session *ClientSession, success bool, message string, remotePort int) {
	resp := &proto.RegisterTunnelResponse{
		Success:    success,
		Message:    message,
		RemotePort: remotePort,
	}
	data, _ := proto.Encode(resp)
	msg := &proto.Message{
		Type: proto.TypeRegisterTunnelResp,
		Data: data,
	}
	session.conn.WriteMessage(msg)
}

// 心跳检测循环
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

// 发送认证响应
func (s *Server) sendAuthResponse(conn *connect.Connect, success bool, message string) {
	resp := &proto.AuthResponse{
		Success: success,
		Message: message,
	}
	data, _ := proto.Encode(resp)
	msg := &proto.Message{
		Type: proto.TypeAuthResp,
		Data: data,
	}
	conn.WriteMessage(msg)
}

// 关闭会话
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

// 检查会话是否已关闭
func (cs *ClientSession) IsClosed() bool {
	select {
	case <-cs.stopCh:
		return true
	default:
		return false
	}
}
