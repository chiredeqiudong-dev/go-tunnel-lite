package proto

import (
	"encoding/json"
	"errors"
)

/*
相关类型常量、错误定义
*/

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
// 当有用户访问公网端口时，服务端发送此消息通知客户端
type NewProxyRequest struct {
	TunnelName string `json:"tunnel_name"`
	ProxyID    string `json:"proxy_id"`
}

// 客户端建立好工作连接后，发送此消息
type ProxyReadyRequest struct {
	ProxyID string `json:"proxy_id"`
}

// 消息类型
const (
	// 认证相关 (0x01-0x0F)
	TypeAuth     uint8 = 0x01 // 客户端 → 服务端：认证请求
	TypeAuthResp uint8 = 0x02 // 服务端 → 客户端：认证响应

	// 隧道管理 (0x10-0x1F)
	TypeRegisterTunnel     uint8 = 0x10 // 客户端 → 服务端：注册隧道
	TypeRegisterTunnelResp uint8 = 0x11 // 服务端 → 客户端：注册隧道响应

	// 代理请求 (0x20-0x2F)
	TypeNewProxy   uint8 = 0x20 // 服务端 → 客户端：通知有新连接
	TypeProxyReady uint8 = 0x21 // 客户端 → 服务端：代理准备就绪

	// 心跳保活 (0x30-0x3F)
	TypePing uint8 = 0x30 // 客户端 → 服务端：心跳请求
	TypePong uint8 = 0x31 // 服务端 → 客户端：心跳响应
)

// 协议类型
const (
	// HeaderLen 消息头长度：Type(1字节) + Length(4字节)
	HeaderLen = 5
	// MaxDataLen 最大消息体长度 64KB, 防止恶意客户端发送超大消息耗尽内存
	MaxDataLen = 64 * 1024
)

// 错误定义
var (
	ErrMsgTooLarge = errors.New("proto: message too large")
	ErrInvalidMsg  = errors.New("proto: invalid message")
)

// GetTypeName 返回消息类型的可读名称（用于日志和调试）
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

// EncodeAuthRequest 编码认证请求
func EncodeAuthRequest(req *AuthRequest) ([]byte, error) {
	return json.Marshal(req)
}

// DecodeAuthRequest 解码认证请求
func DecodeAuthRequest(data []byte) (*AuthRequest, error) {
	req := &AuthRequest{}
	err := json.Unmarshal(data, req)
	return req, err
}

// EncodeAuthResponse 编码认证响应
func EncodeAuthResponse(resp *AuthResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// DecodeAuthResponse 解码认证响应
func DecodeAuthResponse(data []byte) (*AuthResponse, error) {
	resp := &AuthResponse{}
	err := json.Unmarshal(data, resp)
	return resp, err
}

// EncodeRegisterTunnelRequest 编码注册隧道请求
func EncodeRegisterTunnelRequest(req *RegisterTunnelRequest) ([]byte, error) {
	return json.Marshal(req)
}

// DecodeRegisterTunnelRequest 解码注册隧道请求
func DecodeRegisterTunnelRequest(data []byte) (*RegisterTunnelRequest, error) {
	req := &RegisterTunnelRequest{}
	err := json.Unmarshal(data, req)
	return req, err
}

// EncodeRegisterTunnelResponse 编码注册隧道响应
func EncodeRegisterTunnelResponse(resp *RegisterTunnelResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// DecodeRegisterTunnelResponse 解码注册隧道响应
func DecodeRegisterTunnelResponse(data []byte) (*RegisterTunnelResponse, error) {
	resp := &RegisterTunnelResponse{}
	err := json.Unmarshal(data, resp)
	return resp, err
}

// EncodeNewProxyRequest 编码新代理请求
func EncodeNewProxyRequest(req *NewProxyRequest) ([]byte, error) {
	return json.Marshal(req)
}

// DecodeNewProxyRequest 解码新代理请求
func DecodeNewProxyRequest(data []byte) (*NewProxyRequest, error) {
	req := &NewProxyRequest{}
	err := json.Unmarshal(data, req)
	return req, err
}

// EncodeProxyReadyRequest 编码代理就绪请求
func EncodeProxyReadyRequest(req *ProxyReadyRequest) ([]byte, error) {
	return json.Marshal(req)
}

// DecodeProxyReadyRequest 解码代理就绪请求
func DecodeProxyReadyRequest(data []byte) (*ProxyReadyRequest, error) {
	req := &ProxyReadyRequest{}
	err := json.Unmarshal(data, req)
	return req, err
}
