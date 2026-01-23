package connect

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

// 测试基本的消息读写
func TestReadWriteMessage(t *testing.T) {
	// 创建管道模拟 TCP 连接
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	serverConn := WrapConnect(server)
	clientConn := WrapConnect(client)

	// 测试消息
	testMsg := &proto.Message{
		Type: proto.TypeAuth,
		Data: []byte(`{"client_id":"test-client"}`),
	}

	// 启动写入协程
	done := make(chan error, 1)
	go func() {
		done <- clientConn.WriteMessage(testMsg)
	}()

	// 读取消息
	receivedMsg, err := serverConn.ReadMessage()
	if err != nil {
		t.Fatalf("读取消息失败: %v", err)
	}

	// 等待写入完成
	if err := <-done; err != nil {
		t.Fatalf("写入消息失败: %v", err)
	}

	// 验证消息
	if receivedMsg.Type != testMsg.Type {
		t.Errorf("消息类型不匹配: 期望 %d, 实际 %d", testMsg.Type, receivedMsg.Type)
	}
	if string(receivedMsg.Data) != string(testMsg.Data) {
		t.Errorf("消息数据不匹配: 期望 %s, 实际 %s", testMsg.Data, receivedMsg.Data)
	}
}

// 测试并发写入安全性
func TestConcurrentWrite(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	serverConn := WrapConnect(server)
	clientConn := WrapConnect(client)

	msgCount := 100
	var wg sync.WaitGroup

	// 启动读取协程
	received := make(chan *proto.Message, msgCount)
	go func() {
		for i := 0; i < msgCount; i++ {
			msg, err := serverConn.ReadMessage()
			if err != nil {
				t.Errorf("读取消息 %d 失败: %v", i, err)
				return
			}
			received <- msg
		}
		close(received)
	}()

	// 并发写入
	for i := 0; i < msgCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := &proto.Message{
				Type: proto.TypePing,
				Data: []byte{byte(idx)},
			}
			if err := clientConn.WriteMessage(msg); err != nil {
				t.Errorf("写入消息 %d 失败: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	// 验证接收到的消息数量
	count := 0
	for range received {
		count++
	}
	if count != msgCount {
		t.Errorf("消息数量不匹配: 期望 %d, 实际 %d", msgCount, count)
	}
}

// 测试连接关闭
func TestClose(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()

	conn := WrapConnect(client)

	// 关闭前应该是未关闭状态
	if conn.IsClosed() {
		t.Error("新连接不应该处于关闭状态")
	}

	// 第一次关闭
	if err := conn.Close(); err != nil {
		t.Errorf("关闭连接失败: %v", err)
	}

	// 应该标记为已关闭
	if !conn.IsClosed() {
		t.Error("关闭后连接应该处于关闭状态")
	}

	// 重复关闭应该安全（幂等）
	if err := conn.Close(); err != nil {
		t.Errorf("重复关闭不应该返回错误: %v", err)
	}
}

// 测试超时设置
func TestDeadline(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := WrapConnect(client)

	// 设置很短的读取超时
	deadline := time.Now().Add(10 * time.Millisecond)
	if err := conn.SetReadDeadLine(deadline); err != nil {
		t.Fatalf("设置读取超时失败: %v", err)
	}

	// 尝试读取（应该超时）
	_, err := conn.ReadMessage()
	if err == nil {
		t.Error("应该因超时而失败")
	}

	// 检查是否为超时错误
	if netErr, ok := err.(net.Error); ok {
		if !netErr.Timeout() {
			t.Errorf("应该是超时错误，实际: %v", err)
		}
	}
}

// 测试地址获取
func TestAddress(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := WrapConnect(client)

	// net.Pipe 返回的地址是 "pipe"
	if conn.LocalAddr() == nil {
		t.Error("本地地址不应该为 nil")
	}
	if conn.RemoteAddr() == nil {
		t.Error("远程地址不应该为 nil")
	}
}

// 测试获取底层连接
func TestRawConn(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := WrapConnect(client)

	raw := conn.RawConn()
	if raw != client {
		t.Error("RawConn 应该返回原始连接")
	}
}

// 测试空消息
func TestEmptyMessage(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	serverConn := WrapConnect(server)
	clientConn := WrapConnect(client)

	// 空 Data 的消息
	testMsg := &proto.Message{
		Type: proto.TypePong,
		Data: nil,
	}

	done := make(chan error, 1)
	go func() {
		done <- clientConn.WriteMessage(testMsg)
	}()

	receivedMsg, err := serverConn.ReadMessage()
	if err != nil {
		t.Fatalf("读取空消息失败: %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("写入空消息失败: %v", err)
	}

	if receivedMsg.Type != proto.TypePong {
		t.Errorf("消息类型不匹配: 期望 %d, 实际 %d", proto.TypePong, receivedMsg.Type)
	}
	if len(receivedMsg.Data) != 0 {
		t.Errorf("空消息的 Data 应该为空，实际长度: %d", len(receivedMsg.Data))
	}
}
