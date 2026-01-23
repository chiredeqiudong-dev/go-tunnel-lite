package proto

import (
	"encoding/json"
	"errors"
)

/*
1、通信协议、类型、错误定义
2、请求响应编解码
*/

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

const (
	// HeaderLen 消息头长度：Type(1字节) + Length(4字节)
	HeaderLen = 5
	// MaxDataLen 最大消息体长度 64KB, 防止恶意客户端发送超大消息耗尽内存
	MaxDataLen = 64 * 1024
)

// some errors
var (
	ErrMsgTooLarge = errors.New("proto: message too large")
	ErrInvalidMsg  = errors.New("proto: invalid message")
)

// 认证相关
type AuthRequest struct {
	ClientID string `json:"client_id"`
	Token    string `json:"token"`
	Version  string `json:"version"`
}

type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// 隧道管理相关
type TunnelConfig struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	LocalAddr  string `json:"local_addr"`
	RemotePort int    `json:"remote_port"`
}

type RegisterTunnelRequest struct {
	Tunnel TunnelConfig `json:"tunnel"`
}

type RegisterTunnelResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	TunnelName string `json:"tunnel_name"`
	RemotePort int    `json:"remote_port"`
}

// 代理相关
type NewProxyRequest struct {
	TunnelName string `json:"tunnel_name"`
	ProxyID    string `json:"proxy_id"`
}

type ProxyReadyRequest struct {
	ProxyID string `json:"proxy_id"`
}

// GetTypeName 返回消息类型的可读名称
func GetTypeName(t uint8) string {
	switch t {
	case TypeAuth:
		return "Auth"
	case TypeAuthResp:
		return "AuthResp"
	case TypeRegisterTunnel:
		return "RegisterTunnel"
	case TypeRegisterTunnelResp:
		return "RegisterTunnelResp"
	case TypeNewProxy:
		return "NewProxy"
	case TypeProxyReady:
		return "ProxyReady"
	case TypePing:
		return "Ping"
	case TypePong:
		return "Pong"
	default:
		return "Unknown"
	}
}

// Encode 将结构体序列化为 JSON 字节
func Encode[T any](v *T) ([]byte, error) {
	return json.Marshal(v)
}

// Decode 将 JSON 字节反序列化为结构体
func Decode[T any](data []byte) (*T, error) {
	v := new(T)
	err := json.Unmarshal(data, v)
	return v, err
}
