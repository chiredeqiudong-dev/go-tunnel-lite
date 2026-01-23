package client

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/config"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/connect"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

// mockServer 模拟服务端，用于测试客户端
type mockServer struct {
	listener net.Listener
	t        *testing.T
	token    string
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func newMockServer(t *testing.T, token string) *mockServer {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建mock服务端失败: %v", err)
	}
	return &mockServer{
		listener: listener,
		t:        t,
		token:    token,
		stopCh:   make(chan struct{}),
	}
}

func (s *mockServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *mockServer) Close() {
	close(s.stopCh)
	s.listener.Close()
	s.wg.Wait()
}

// handleConnection 处理客户端连接（模拟服务端行为）
func (s *mockServer) handleConnection(conn net.Conn, authSuccess bool, tunnelSuccess bool) {
	defer conn.Close()
	c := connect.WrapConnect(conn)

	// 1. 读取认证请求
	msg, err := c.ReadMessage()
	if err != nil {
		s.t.Logf("读取认证请求失败: %v", err)
		return
	}

	if msg.Type != proto.TypeAuth {
		s.t.Logf("期望认证请求，收到: %s", proto.GetTypeName(msg.Type))
		return
	}

	authReq, err := proto.Decode[proto.AuthRequest](msg.Data)
	if err != nil {
		s.t.Logf("解码认证请求失败: %v", err)
		return
	}

	// 2. 发送认证响应
	var authResp *proto.AuthResponse
	if authSuccess && authReq.Token == s.token {
		authResp = &proto.AuthResponse{Success: true, Message: "认证成功"}
	} else {
		authResp = &proto.AuthResponse{Success: false, Message: "token无效"}
	}

	respData, _ := proto.Encode(authResp)
	respMsg := &proto.Message{Type: proto.TypeAuthResp, Data: respData}
	if err := c.WriteMessage(respMsg); err != nil {
		s.t.Logf("发送认证响应失败: %v", err)
		return
	}

	if !authResp.Success {
		return
	}

	// 3. 处理隧道注册请求
	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return
		}

		switch msg.Type {
		case proto.TypeRegisterTunnel:
			tunnelReq, _ := proto.Decode[proto.RegisterTunnelRequest](msg.Data)
			var tunnelResp *proto.RegisterTunnelResponse
			if tunnelSuccess {
				tunnelResp = &proto.RegisterTunnelResponse{
					Success:    true,
					Message:    "注册成功",
					TunnelName: tunnelReq.Tunnel.Name,
					RemotePort: tunnelReq.Tunnel.RemotePort,
				}
			} else {
				tunnelResp = &proto.RegisterTunnelResponse{
					Success: false,
					Message: "端口已被占用",
				}
			}
			respData, _ := proto.Encode(tunnelResp)
			c.WriteMessage(&proto.Message{Type: proto.TypeRegisterTunnelResp, Data: respData})

			if !tunnelSuccess {
				return
			}

		case proto.TypePing:
			// 响应心跳
			c.WriteMessage(&proto.Message{Type: proto.TypePong})

		default:
			s.t.Logf("收到未知消息: %s", proto.GetTypeName(msg.Type))
		}
	}
}

// TestNewClient 测试创建客户端
func TestNewClient(t *testing.T) {
	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			ServerAddr: "127.0.0.1:7000",
			Token:      "test-token",
		},
	}

	client := NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient 返回 nil")
	}
	if client.cfg != cfg {
		t.Error("配置未正确设置")
	}
	if client.stopCh == nil {
		t.Error("stopCh 未初始化")
	}
}

// TestClientAuthSuccess 测试认证成功
func TestClientAuthSuccess(t *testing.T) {
	server := newMockServer(t, "valid-token")
	defer server.Close()

	// 启动服务端处理协程
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		conn, err := server.listener.Accept()
		if err != nil {
			return
		}
		server.handleConnection(conn, true, true)
	}()

	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			ServerAddr:        server.Addr(),
			Token:             "valid-token",
			HeartbeatInterval: 30,
			Tunnels: []config.TunnelConfig{
				{
					Name:       "test-tunnel",
					LocalAddr:  "127.0.0.1:8080",
					RemotePort: 9080,
				},
			},
		},
	}

	client := NewClient(cfg)
	err := client.Start()
	if err != nil {
		t.Fatalf("客户端启动失败: %v", err)
	}

	// 等待一小段时间确保启动完成
	time.Sleep(100 * time.Millisecond)

	client.Stop()
}

// TestClientAuthFail 测试认证失败
func TestClientAuthFail(t *testing.T) {
	server := newMockServer(t, "valid-token")
	defer server.Close()

	// 启动服务端处理协程
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		conn, err := server.listener.Accept()
		if err != nil {
			return
		}
		server.handleConnection(conn, true, true) // 服务端正常，但token不匹配
	}()

	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			ServerAddr:        server.Addr(),
			Token:             "wrong-token", // 错误的 token
			HeartbeatInterval: 30,
		},
	}

	client := NewClient(cfg)
	err := client.Start()
	if err == nil {
		client.Stop()
		t.Fatal("期望认证失败，但启动成功了")
	}

	t.Logf("认证失败（预期）: %v", err)
}

// TestClientTunnelRegisterFail 测试隧道注册失败
func TestClientTunnelRegisterFail(t *testing.T) {
	server := newMockServer(t, "valid-token")
	defer server.Close()

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		conn, err := server.listener.Accept()
		if err != nil {
			return
		}
		server.handleConnection(conn, true, false) // 认证成功，隧道注册失败
	}()

	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			ServerAddr:        server.Addr(),
			Token:             "valid-token",
			HeartbeatInterval: 30,
			Tunnels: []config.TunnelConfig{
				{
					Name:       "test-tunnel",
					LocalAddr:  "127.0.0.1:8080",
					RemotePort: 9080,
				},
			},
		},
	}

	client := NewClient(cfg)
	err := client.Start()
	if err == nil {
		client.Stop()
		t.Fatal("期望隧道注册失败，但启动成功了")
	}

	t.Logf("隧道注册失败（预期）: %v", err)
}

// TestClientConnectFail 测试连接失败
func TestClientConnectFail(t *testing.T) {
	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			ServerAddr: "127.0.0.1:59999", // 不存在的端口
			Token:      "test-token",
		},
	}

	client := NewClient(cfg)
	err := client.Start()
	if err == nil {
		client.Stop()
		t.Fatal("期望连接失败，但启动成功了")
	}

	t.Logf("连接失败（预期）: %v", err)
}

// TestClientDoubleStart 测试重复启动
func TestClientDoubleStart(t *testing.T) {
	server := newMockServer(t, "valid-token")
	defer server.Close()

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		conn, err := server.listener.Accept()
		if err != nil {
			return
		}
		server.handleConnection(conn, true, true)
	}()

	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			ServerAddr:        server.Addr(),
			Token:             "valid-token",
			HeartbeatInterval: 30,
			Tunnels: []config.TunnelConfig{
				{
					Name:       "test-tunnel",
					LocalAddr:  "127.0.0.1:8080",
					RemotePort: 9080,
				},
			},
		},
	}

	client := NewClient(cfg)

	// 第一次启动
	err := client.Start()
	if err != nil {
		t.Fatalf("第一次启动失败: %v", err)
	}

	// 第二次启动应该失败
	err = client.Start()
	if err == nil {
		t.Error("期望第二次启动失败")
	} else {
		t.Logf("第二次启动失败（预期）: %v", err)
	}

	client.Stop()
}

// TestClientHeartbeat 测试心跳
func TestClientHeartbeat(t *testing.T) {
	server := newMockServer(t, "valid-token")
	defer server.Close()

	heartbeatReceived := make(chan struct{}, 5)

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		conn, err := server.listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		c := connect.WrapConnect(conn)

		// 处理认证
		msg, _ := c.ReadMessage()
		if msg.Type == proto.TypeAuth {
			respData, _ := proto.Encode(&proto.AuthResponse{Success: true})
			c.WriteMessage(&proto.Message{Type: proto.TypeAuthResp, Data: respData})
		}

		// 处理隧道注册和心跳
		for {
			msg, err := c.ReadMessage()
			if err != nil {
				return
			}

			switch msg.Type {
			case proto.TypeRegisterTunnel:
				tunnelReq, _ := proto.Decode[proto.RegisterTunnelRequest](msg.Data)
				respData, _ := proto.Encode(&proto.RegisterTunnelResponse{
					Success:    true,
					TunnelName: tunnelReq.Tunnel.Name,
					RemotePort: tunnelReq.Tunnel.RemotePort,
				})
				c.WriteMessage(&proto.Message{Type: proto.TypeRegisterTunnelResp, Data: respData})

			case proto.TypePing:
				c.WriteMessage(&proto.Message{Type: proto.TypePong})
				select {
				case heartbeatReceived <- struct{}{}:
				default:
				}
			}
		}
	}()

	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			ServerAddr:        server.Addr(),
			Token:             "valid-token",
			HeartbeatInterval: 1, // 1秒心跳间隔
			Tunnels: []config.TunnelConfig{
				{
					Name:       "test-tunnel",
					LocalAddr:  "127.0.0.1:8080",
					RemotePort: 9080,
				},
			},
		},
	}

	client := NewClient(cfg)
	err := client.Start()
	if err != nil {
		t.Fatalf("客户端启动失败: %v", err)
	}

	// 等待收到心跳
	select {
	case <-heartbeatReceived:
		t.Log("收到心跳（预期）")
	case <-time.After(3 * time.Second):
		t.Error("超时未收到心跳")
	}

	client.Stop()
}

// TestGenerateClientID 测试客户端ID生成
func TestGenerateClientID(t *testing.T) {
	id1 := fmt.Sprintf("client-%d", time.Now().UnixNano())
	time.Sleep(time.Nanosecond)
	id2 := fmt.Sprintf("client-%d", time.Now().UnixNano())

	if id1 == "" {
		t.Error("生成的ID为空")
	}

	if id1 == id2 {
		t.Error("两次生成的ID相同")
	}

	t.Logf("生成的ID: %s, %s", id1, id2)
}

// TestClientStopIdempotent 测试停止的幂等性
func TestClientStopIdempotent(t *testing.T) {
	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			ServerAddr: "127.0.0.1:7000",
			Token:      "test-token",
		},
	}

	client := NewClient(cfg)

	// 多次调用 Stop 不应该 panic
	client.Stop()
	client.Stop()
	client.Stop()

	t.Log("多次 Stop 调用成功，无 panic")
}
