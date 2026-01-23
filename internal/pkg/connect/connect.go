package connect

import (
	"net"
	"sync"
	"time"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

/*
TCP 连接封装
提供消息级别的读写、超时处理、优雅关闭等功能
*/

// 提供消息级别的读写，支持并发安全
type Connect struct {
	conn net.Conn
	// reader *bufio.Reader

	writeMu sync.Mutex
	readMu  sync.Mutex

	closed   bool
	closedMu sync.Mutex
}

// 将原生 net.Conn 封装为 Connect
func WrapConnect(c net.Conn) *Connect {
	return &Connect{
		conn: c,
		// reader: bufio.NewReader(c),
	}
}

// 阻塞直到读取到完整消息或发生错误
func (c *Connect) ReadMessage() (*proto.Message, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	// 解码
	msg := &proto.Message{}
	_, err := msg.ReadFrom(c.conn)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// 写入一条消息
func (c *Connect) WriteMessage(msg *proto.Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// 编码
	_, err := msg.WriteTo(c.conn)
	return err
}

// 设置读取超时
func (c *Connect) SetReadDeadLine(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// 设置写入超时
func (c *Connect) SetWriteDeadLine(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// 设置读写超时
func (c *Connect) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// 关闭连接
func (c *Connect) Close() error {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	return c.conn.Close()
}

// 检查连接是否已关闭
func (c *Connect) IsClosed() bool {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()

	return c.closed
}

// 获取远程地址
func (c *Connect) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// 获取本地地址
func (c *Connect) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// 获取底层连接（用于特殊场景，如数据转发）
func (c *Connect) RawConn() net.Conn {
	return c.conn
}
