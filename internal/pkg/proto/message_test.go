package proto

import (
	"bytes"
	"testing"
)

// TestMessageWriteAndRead 测试消息的写入和读取
// 这是一个"往返测试"：写入 → 读取 → 验证数据一致
func TestMessageWriteAndRead(t *testing.T) {
	// 准备测试数据
	original := &Message{
		Type: TypeAuth,
		Data: []byte(`{"token":"secret123","version":"1.0.0"}`),
	}

	// 使用 bytes.Buffer 模拟网络连接
	// bytes.Buffer 实现了 io.Reader 和 io.Writer 接口
	// 非常适合用于测试，不需要真正的网络连接
	buf := &bytes.Buffer{}

	// 写入消息
	n, err := original.WriteTo(buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err) // Fatalf 会立即终止测试
	}
	t.Logf("写入 %d 字节", n)

	// 读取消息
	received := &Message{}
	n, err = received.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}
	t.Logf("读取 %d 字节", n)

	// 验证消息类型
	if received.Type != original.Type {
		t.Errorf("Type mismatch: got %d, want %d", received.Type, original.Type)
	}

	// 验证消息内容
	if !bytes.Equal(received.Data, original.Data) {
		t.Errorf("Data mismatch: got %s, want %s", received.Data, original.Data)
	}
}

// TestMessageWithPayload 测试使用 NewMessage 和 Unmarshal
func TestMessageWithPayload(t *testing.T) {
	// 1. 创建认证请求消息
	authReq := &AuthRequest{
		Token:   "my-secret-token",
		Version: "1.0.0",
	}

	msg, err := NewMessage(TypeAuth, authReq)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	t.Logf("消息类型: %s", GetTypeName(msg.Type))
	t.Logf("消息内容: %s", string(msg.Data))

	// 2. 模拟网络传输
	buf := &bytes.Buffer{}
	_, err = msg.WriteTo(buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// 3. 读取消息
	received := &Message{}
	_, err = received.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}

	// 4. 反序列化
	var parsedReq AuthRequest
	err = received.Unmarshal(&parsedReq)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// 5. 验证
	if parsedReq.Token != authReq.Token {
		t.Errorf("Token mismatch: got %s, want %s", parsedReq.Token, authReq.Token)
	}
	if parsedReq.Version != authReq.Version {
		t.Errorf("Version mismatch: got %s, want %s", parsedReq.Version, authReq.Version)
	}

	t.Logf("解析成功: Token=%s, Version=%s", parsedReq.Token, parsedReq.Version)
}

// TestEmptyMessage 测试空消息（如心跳）
func TestEmptyMessage(t *testing.T) {
	// Ping 消息通常没有消息体
	msg := &Message{Type: TypePing}

	buf := &bytes.Buffer{}
	_, err := msg.WriteTo(buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// 验证只写入了消息头（5字节）
	if buf.Len() != HeaderLen {
		t.Errorf("Expected %d bytes, got %d", HeaderLen, buf.Len())
	}

	// 读取
	received := &Message{}
	_, err = received.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}

	if received.Type != TypePing {
		t.Errorf("Type mismatch: got %d, want %d", received.Type, TypePing)
	}
	if len(received.Data) != 0 {
		t.Errorf("Expected empty data, got %d bytes", len(received.Data))
	}
}

// TestMessageTooLarge 测试消息过大的错误处理
func TestMessageTooLarge(t *testing.T) {
	// 创建一个超大消息
	largeData := make([]byte, MaxDataLen+1)
	msg := &Message{
		Type: TypeAuth,
		Data: largeData,
	}

	buf := &bytes.Buffer{}
	_, err := msg.WriteTo(buf)

	// 期望返回错误
	if err != ErrMsgTooLarge {
		t.Errorf("Expected ErrMsgTooLarge, got %v", err)
	}
}

// TestGetTypeName 测试类型名称转换
func TestGetTypeName(t *testing.T) {
	tests := []struct {
		msgType  uint8
		expected string
	}{
		{TypeAuth, "Auth"},
		{TypeAuthResp, "AuthResp"},
		{TypePing, "Ping"},
		{TypePong, "Pong"},
		{0xFF, "Unknown"},
	}

	// 表格驱动测试（Table-driven tests）
	// Go 社区推荐的测试风格，便于添加更多测试用例
	for _, tt := range tests {
		got := GetTypeName(tt.msgType)
		if got != tt.expected {
			t.Errorf("GetTypeName(%d) = %s, want %s", tt.msgType, got, tt.expected)
		}
	}
}
