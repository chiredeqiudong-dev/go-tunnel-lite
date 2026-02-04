package connect

import (
	"net"
	"syscall"
	"time"
)

const (
	// DefaultKeepAliveInterval 默认 Keep-Alive 间隔
	DefaultKeepAliveInterval = 30 * time.Second
	// DefaultKeepAliveTimeout 默认 Keep-Alive 超时
	DefaultKeepAliveTimeout = 5 * time.Second
	// DefaultKeepAliveCount 默认 Keep-Alive 探测次数
	DefaultKeepAliveCount = 3
)

// SetTCPKeepAlive 设置 TCP Keep-Alive 参数
func SetTCPKeepAlive(conn net.Conn) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}

	err := tcpConn.SetKeepAlive(true)
	if err != nil {
		return err
	}

	err = tcpConn.SetKeepAlivePeriod(DefaultKeepAliveInterval)
	if err != nil {
		return err
	}

	return nil
}

// SetTCPSocketOptions 设置优化过的 TCP Socket 参数
func SetTCPSocketOptions(conn net.Conn) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}

	// 启用 TCP Keep-Alive
	if err := SetTCPKeepAlive(tcpConn); err != nil {
		return err
	}

	// 禁用 Nagle 算法（对于低延迟场景）
	if err := tcpConn.SetNoDelay(true); err != nil {
		return err
	}

	return nil
}

// SetSysTCPKeepAlive 使用系统调用设置 Keep-Alive 参数（Linux 专用）
func SetSysTCPKeepAlive(conn *net.TCPConn, interval, timeout time.Duration, count int) error {
	file, err := conn.File()
	if err != nil {
		return err
	}
	defer file.Close()

	fd := int(file.Fd())

	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, 1); err != nil {
		return err
	}

	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPIDLE, int(interval.Seconds())); err != nil {
		return err
	}

	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPINTVL, int(timeout.Seconds())); err != nil {
		return err
	}

	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPCNT, count); err != nil {
		return err
	}

	return nil
}
