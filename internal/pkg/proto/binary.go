package proto

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

// BinaryEncoder 二进制编码器接口
type BinaryEncoder interface {
	EncodeBinary() ([]byte, error)
}

// BinaryDecoder 二进制解码器接口
type BinaryDecoder interface {
	DecodeBinary(data []byte) error
}

// BinaryMessage 二进制消息接口
type BinaryMessage interface {
	BinaryEncoder
	BinaryDecoder
}

// 二进制协议常量
const (
	// 字符串最大长度（2字节表示长度）
	MaxStringLen = 65535
)

// 二进制编码错误
var (
	ErrStringTooLong = errors.New("proto: string too long")
)

// stringBufferPool 用于重用字符串编码缓冲区
var stringBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 128) // 预分配128字节容量
	},
}

// encodeBufferPool 用于重用一般编码缓冲区
var encodeBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 256) // 预分配256字节容量
	},
}

// encodeString 编码字符串（长度前缀）
func encodeString(s string) []byte {
	length := len(s)
	if length > MaxStringLen {
		length = MaxStringLen
	}

	// 暂时禁用内存池，避免数据污染
	data := make([]byte, 2+length)
	binary.BigEndian.PutUint16(data[0:2], uint16(length))
	copy(data[2:], s[:length])

	return data
}

// decodeString 解码字符串（长度前缀）
func decodeString(data []byte) (string, int, error) {
	if len(data) < 2 {
		return "", 0, io.ErrUnexpectedEOF
	}

	length := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+length {
		return "", 0, io.ErrUnexpectedEOF
	}

	return string(data[2 : 2+length]), 2 + length, nil
}

// encodeBool 编码布尔值
func encodeBool(b bool) []byte {
	if b {
		return []byte{1}
	}
	return []byte{0}
}

// getEncodeBuffer 从内存池获取编码缓冲区（暂时禁用内存池）
func getEncodeBuffer(size int) []byte {
	// 暂时禁用内存池，避免数据污染问题
	return make([]byte, size)
}

// putEncodeBuffer 将编码缓冲区归还到内存池（暂时禁用内存池）
func putEncodeBuffer(buf []byte) {
	// 暂时禁用内存池
}

// decodeBool 解码布尔值
func decodeBool(data []byte) (bool, error) {
	if len(data) < 1 {
		return false, io.ErrUnexpectedEOF
	}
	return data[0] == 1, nil
}

// AuthRequest 二进制编码实现
func (r *AuthRequest) EncodeBinary() ([]byte, error) {
	clientIDData := encodeString(r.ClientID)
	tokenData := encodeString(r.Token)
	versionData := encodeString(r.Version)

	// 计算总长度
	totalLen := len(clientIDData) + len(tokenData) + len(versionData)

	// 从内存池获取缓冲区
	data := getEncodeBuffer(totalLen)
	defer putEncodeBuffer(data)

	// 拼接数据
	offset := 0
	copy(data[offset:], clientIDData)
	offset += len(clientIDData)
	copy(data[offset:], tokenData)
	offset += len(tokenData)
	copy(data[offset:], versionData)

	return data, nil
}

// AuthRequest 二进制解码实现
func (r *AuthRequest) DecodeBinary(data []byte) error {
	var offset int
	var err error

	// 解码 ClientID
	r.ClientID, offset, err = decodeString(data)
	if err != nil {
		return err
	}

	// 解码 Token
	var token string
	token, n, err := decodeString(data[offset:])
	if err != nil {
		return err
	}
	r.Token = token
	offset += n

	// 解码 Version
	r.Version, _, err = decodeString(data[offset:])
	return err
}

// AuthResponse 二进制编码实现
func (r *AuthResponse) EncodeBinary() ([]byte, error) {
	successData := encodeBool(r.Success)
	messageData := encodeString(r.Message)

	totalLen := len(successData) + len(messageData)

	// 从内存池获取缓冲区
	data := getEncodeBuffer(totalLen)
	defer putEncodeBuffer(data)

	offset := 0
	copy(data[offset:], successData)
	offset += len(successData)
	copy(data[offset:], messageData)

	return data, nil
}

// AuthResponse 二进制解码实现
func (r *AuthResponse) DecodeBinary(data []byte) error {
	var offset int
	var err error

	// 解码 Success
	r.Success, err = decodeBool(data)
	if err != nil {
		return err
	}
	offset += 1

	// 解码 Message
	r.Message, _, err = decodeString(data[offset:])
	return err
}

// TunnelConfig 二进制编码实现
func (t *TunnelConfig) EncodeBinary() ([]byte, error) {
	nameData := encodeString(t.Name)
	typeData := encodeString(t.Type)
	localAddrData := encodeString(t.LocalAddr)
	remotePortData := make([]byte, 4)
	binary.BigEndian.PutUint32(remotePortData, uint32(t.RemotePort))

	totalLen := len(nameData) + len(typeData) + len(localAddrData) + len(remotePortData)

	// 从内存池获取缓冲区
	data := getEncodeBuffer(totalLen)
	defer putEncodeBuffer(data)

	offset := 0
	copy(data[offset:], nameData)
	offset += len(nameData)
	copy(data[offset:], typeData)
	offset += len(typeData)
	copy(data[offset:], localAddrData)
	offset += len(localAddrData)
	copy(data[offset:], remotePortData)

	return data, nil
}

// TunnelConfig 二进制解码实现
func (t *TunnelConfig) DecodeBinary(data []byte) error {
	var offset int
	var err error

	// 解码 Name
	t.Name, offset, err = decodeString(data)
	if err != nil {
		return err
	}

	// 解码 Type
	var tunnelType string
	tunnelType, n, err := decodeString(data[offset:])
	if err != nil {
		return err
	}
	t.Type = tunnelType
	offset += n

	// 解码 LocalAddr
	t.LocalAddr, n, err = decodeString(data[offset:])
	if err != nil {
		return err
	}
	offset += n

	// 解码 RemotePort
	if len(data[offset:]) < 4 {
		return io.ErrUnexpectedEOF
	}
	t.RemotePort = int(binary.BigEndian.Uint32(data[offset : offset+4]))

	return nil
}

// RegisterTunnelRequest 二进制编码实现
func (r *RegisterTunnelRequest) EncodeBinary() ([]byte, error) {
	return r.Tunnel.EncodeBinary()
}

// RegisterTunnelRequest 二进制解码实现
func (r *RegisterTunnelRequest) DecodeBinary(data []byte) error {
	return r.Tunnel.DecodeBinary(data)
}

// RegisterTunnelResponse 二进制编码实现
func (r *RegisterTunnelResponse) EncodeBinary() ([]byte, error) {
	successData := encodeBool(r.Success)
	messageData := encodeString(r.Message)
	tunnelNameData := encodeString(r.TunnelName)
	remotePortData := make([]byte, 4)
	binary.BigEndian.PutUint32(remotePortData, uint32(r.RemotePort))

	totalLen := len(successData) + len(messageData) + len(tunnelNameData) + len(remotePortData)
	data := make([]byte, totalLen)

	offset := 0
	copy(data[offset:], successData)
	offset += len(successData)
	copy(data[offset:], messageData)
	offset += len(messageData)
	copy(data[offset:], tunnelNameData)
	offset += len(tunnelNameData)
	copy(data[offset:], remotePortData)

	return data, nil
}

// RegisterTunnelResponse 二进制解码实现
func (r *RegisterTunnelResponse) DecodeBinary(data []byte) error {
	var offset int
	var err error

	// 解码 Success
	r.Success, err = decodeBool(data)
	if err != nil {
		return err
	}
	offset += 1

	// 解码 Message
	var message string
	message, n, err := decodeString(data[offset:])
	if err != nil {
		return err
	}
	r.Message = message
	offset += n

	// 解码 TunnelName
	r.TunnelName, n, err = decodeString(data[offset:])
	if err != nil {
		return err
	}
	offset += n

	// 解码 RemotePort
	if len(data[offset:]) < 4 {
		return io.ErrUnexpectedEOF
	}
	r.RemotePort = int(binary.BigEndian.Uint32(data[offset : offset+4]))

	return nil
}

// NewProxyRequest 二进制编码实现
func (r *NewProxyRequest) EncodeBinary() ([]byte, error) {
	tunnelNameData := encodeString(r.TunnelName)
	proxyIDData := encodeString(r.ProxyID)

	totalLen := len(tunnelNameData) + len(proxyIDData)
	data := make([]byte, totalLen)

	offset := 0
	copy(data[offset:], tunnelNameData)
	offset += len(tunnelNameData)
	copy(data[offset:], proxyIDData)

	return data, nil
}

// NewProxyRequest 二进制解码实现
func (r *NewProxyRequest) DecodeBinary(data []byte) error {
	var offset int
	var err error

	// 解码 TunnelName
	r.TunnelName, offset, err = decodeString(data)
	if err != nil {
		return err
	}

	// 解码 ProxyID
	r.ProxyID, _, err = decodeString(data[offset:])
	return err
}

// ProxyReadyRequest 二进制编码实现
func (r *ProxyReadyRequest) EncodeBinary() ([]byte, error) {
	return encodeString(r.ProxyID), nil
}

// ProxyReadyRequest 二进制解码实现
func (r *ProxyReadyRequest) DecodeBinary(data []byte) error {
	proxyID, _, err := decodeString(data)
	if err != nil {
		return err
	}
	r.ProxyID = proxyID
	return nil
}

// EncodeBinary 通用二进制编码函数
func EncodeBinary(msg BinaryMessage) ([]byte, error) {
	return msg.EncodeBinary()
}

// DecodeBinary 通用二进制解码函数
func DecodeBinary[T BinaryMessage](data []byte, msg T) error {
	return msg.DecodeBinary(data)
}

// EncodeMixed 混合编码（优先二进制，回退JSON）
func EncodeMixed(v interface{}) ([]byte, error) {
	if msg, ok := v.(BinaryMessage); ok {
		return msg.EncodeBinary()
	}
	return json.Marshal(v)
}

// DecodeMixed 混合解码（优先二进制，回退JSON）
func DecodeMixed[T any](data []byte) (*T, error) {
	v := new(T)

	// 尝试二进制解码
	if msg, ok := interface{}(v).(BinaryMessage); ok {
		err := msg.DecodeBinary(data)
		if err == nil {
			return v, nil
		}
	}

	// 回退到 JSON 解码
	err := json.Unmarshal(data, v)
	return v, err
}
