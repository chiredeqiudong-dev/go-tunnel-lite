package server

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/config"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/connect"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

// 创建测试配置
func newTestServerConfig(port int) *config.ServerConfig {
	return &config.ServerConfig{
		Server: config.ServerSettings{
			ControlAddr:       fmt.Sprintf("127.0.0.1:%d", port),
			Token:             "test-token",
			HeartbeatInterval: 1 * time.Second,
			HeartbeatTimeout:  3 * time.Second,
		},
	}
}

// TestServerStartStop 测试服务端启动和停止
func TestServerStartStop(t *testing.T) {
	cfg := newTestServerConfig(17000)

	s := NewServer(cfg)

	// 启动服务端
	if err := s.Start(); err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}

	// 确认监听器已创建
	if s.listener == nil {
		t.Fatal("监听器未创建")
	}

	// 停止服务端
	s.Stop()

	// 确认停止后无法连接
	_, err := net.DialTimeout("tcp", "127.0.0.1:17000", time.Second)
	if err == nil {
		t.Fatal("服务端停止后仍可连接")
	}
}

// TestClientAuth 测试客户端认证
func TestClientAuth(t *testing.T) {
	cfg := newTestServerConfig(17001)

	s := NewServer(cfg)
	if err := s.Start(); err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer s.Stop()

	// 等待服务端就绪
	time.Sleep(100 * time.Millisecond)

	// 连接服务端
	rawConn, err := net.Dial("tcp", "127.0.0.1:17001")
	if err != nil {
		t.Fatalf("连接服务端失败: %v", err)
	}
	conn := connect.WrapConnect(rawConn)
	defer conn.Close()

	// 发送认证请求
	authReq := &proto.AuthRequest{
		ClientID: "test-client",
		Token:    "test-token",
	}
	data, _ := proto.EncodeAuthRequest(authReq)
	msg := &proto.Message{
		Type: proto.TypeAuth,
		Data: data,
	}
	if err := conn.WriteMessage(msg); err != nil {
		t.Fatalf("发送认证消息失败: %v", err)
	}

	// 读取认证响应
	respMsg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("读取认证响应失败: %v", err)
	}

	if respMsg.Type != proto.TypeAuthResp {
		t.Fatalf("期望 TypeAuthResp，收到: %d", respMsg.Type)
	}

	authResp, err := proto.DecodeAuthResponse(respMsg.Data)
	if err != nil {
		t.Fatalf("解析认证响应失败: %v", err)
	}

	if !authResp.Success {
		t.Fatalf("认证失败: %s", authResp.Message)
	}

	t.Logf("认证成功: %s", authResp.Message)

	// 验证会话已创建
	time.Sleep(100 * time.Millisecond)
	s.sessionsMu.RLock()
	_, exists := s.sessions["test-client"]
	s.sessionsMu.RUnlock()

	if !exists {
		t.Fatal("会话未创建")
	}
}

// TestClientAuthFail 测试认证失败
func TestClientAuthFail(t *testing.T) {
	cfg := newTestServerConfig(17002)

	s := NewServer(cfg)
	if err := s.Start(); err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	// 连接服务端
	rawConn, err := net.Dial("tcp", "127.0.0.1:17002")
	if err != nil {
		t.Fatalf("连接服务端失败: %v", err)
	}
	conn := connect.WrapConnect(rawConn)
	defer conn.Close()

	// 发送错误的 Token
	authReq := &proto.AuthRequest{
		ClientID: "test-client",
		Token:    "wrong-token",
	}
	data, _ := proto.EncodeAuthRequest(authReq)
	msg := &proto.Message{
		Type: proto.TypeAuth,
		Data: data,
	}
	if err := conn.WriteMessage(msg); err != nil {
		t.Fatalf("发送认证消息失败: %v", err)
	}

	// 读取认证响应
	respMsg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("读取认证响应失败: %v", err)
	}

	authResp, _ := proto.DecodeAuthResponse(respMsg.Data)
	if authResp.Success {
		t.Fatal("错误的 Token 应该认证失败")
	}

	t.Logf("预期的认证失败: %s", authResp.Message)
}

// TestHeartbeat 测试心跳
func TestHeartbeat(t *testing.T) {
	cfg := newTestServerConfig(17003)

	s := NewServer(cfg)
	if err := s.Start(); err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	// 连接并认证
	rawConn, err := net.Dial("tcp", "127.0.0.1:17003")
	if err != nil {
		t.Fatalf("连接服务端失败: %v", err)
	}
	conn := connect.WrapConnect(rawConn)
	defer conn.Close()

	// 认证
	authReq := &proto.AuthRequest{
		ClientID: "heartbeat-client",
		Token:    "test-token",
	}
	data, _ := proto.EncodeAuthRequest(authReq)
	msg := &proto.Message{
		Type: proto.TypeAuth,
		Data: data,
	}
	conn.WriteMessage(msg)
	conn.ReadMessage() // 读取认证响应

	// 等待并响应心跳
	for i := 0; i < 3; i++ {
		// 等待服务端发送 Ping
		pingMsg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("读取 Ping 失败: %v", err)
		}

		if pingMsg.Type != proto.TypePing {
			t.Fatalf("期望 Ping，收到: %d", pingMsg.Type)
		}

		// 响应 Pong
		pong := &proto.Message{Type: proto.TypePong}
		if err := conn.WriteMessage(pong); err != nil {
			t.Fatalf("发送 Pong 失败: %v", err)
		}

		t.Logf("心跳 #%d 完成", i+1)
	}
}

// TestDuplicateClient 测试重复客户端连接
func TestDuplicateClient(t *testing.T) {
	cfg := newTestServerConfig(17004)

	s := NewServer(cfg)
	if err := s.Start(); err != nil {
		t.Fatalf("启动服务端失败: %v", err)
	}
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)

	// 第一个客户端连接
	rawConn1, _ := net.Dial("tcp", "127.0.0.1:17004")
	conn1 := connect.WrapConnect(rawConn1)

	authReq := &proto.AuthRequest{
		ClientID: "duplicate-client",
		Token:    "test-token",
	}
	data, _ := proto.EncodeAuthRequest(authReq)
	msg := &proto.Message{Type: proto.TypeAuth, Data: data}
	conn1.WriteMessage(msg)
	conn1.ReadMessage()

	// 验证第一个客户端已注册
	time.Sleep(100 * time.Millisecond)
	s.sessionsMu.RLock()
	session1 := s.sessions["duplicate-client"]
	s.sessionsMu.RUnlock()
	if session1 == nil {
		t.Fatal("第一个客户端会话未创建")
	}

	// 第二个客户端使用相同 ID 连接
	rawConn2, _ := net.Dial("tcp", "127.0.0.1:17004")
	conn2 := connect.WrapConnect(rawConn2)
	defer conn2.Close()

	conn2.WriteMessage(msg)
	conn2.ReadMessage()

	// 等待服务端处理
	time.Sleep(100 * time.Millisecond)

	// 验证第一个连接已被关闭
	if !session1.IsClosed() {
		t.Fatal("旧会话应该被关闭")
	}

	// 验证新会话已创建
	s.sessionsMu.RLock()
	session2 := s.sessions["duplicate-client"]
	s.sessionsMu.RUnlock()
	if session2 == nil || session2.IsClosed() {
		t.Fatal("新会话应该存在且未关闭")
	}

	t.Log("重复客户端处理正确：旧连接已关闭，新连接已建立")
}
