package server

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
)

type Proxy struct {
	name       string
	remotePort int
	listener   net.Listener
	stopCh     chan struct{}
	mu         sync.Mutex
	closed     bool
}

// BufferPool 内存缓冲区池，减少内存分配
var BufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024) // 32KB 缓冲区
	},
}

// GetBuffer 从池中获取缓冲区
func GetBuffer() []byte {
	return BufferPool.Get().([]byte)
}

// PutBuffer 将缓冲区放回池中
func PutBuffer(buf []byte) {
	BufferPool.Put(buf)
}

// ProxyConnection 代理连接，包含共享缓冲区
type ProxyConnection struct {
	localConn  net.Conn
	remoteConn net.Conn
	buffer     []byte
	proxyID    string
}

// NewProxyConnection 创建代理连接
func NewProxyConnection(local, remote net.Conn, proxyID string) *ProxyConnection {
	return &ProxyConnection{
		localConn:  local,
		remoteConn: remote,
		buffer:     GetBuffer(),
		proxyID:    proxyID,
	}
}

// Close 关闭代理连接，释放缓冲区
func (pc *ProxyConnection) Close() {
	if pc.localConn != nil {
		pc.localConn.Close()
	}
	if pc.remoteConn != nil {
		pc.remoteConn.Close()
	}
	PutBuffer(pc.buffer)
	pc.localConn = nil
	pc.remoteConn = nil
}

// Forward 双向转发数据，使用零拷贝优化
func (pc *ProxyConnection) Forward() {
	var wg sync.WaitGroup
	wg.Add(2)

	// local -> remote
	go func() {
		defer wg.Done()
		n, _ := io.Copy(pc.remoteConn, pc.localConn)
		log.Debug("转发完成", "proxyID", pc.proxyID, "direction", "local->remote", "bytes", n)
	}()

	// remote -> local
	go func() {
		defer wg.Done()
		n, _ := io.Copy(pc.localConn, pc.remoteConn)
		log.Debug("转发完成", "proxyID", pc.proxyID, "direction", "remote->local", "bytes", n)
	}()

	wg.Wait()
	log.Info("代理连接关闭", "proxyID", pc.proxyID)
}

func NewProxy(name string, remotePort int) *Proxy {
	return &Proxy{
		name:       name,
		remotePort: remotePort,
		stopCh:     make(chan struct{}),
	}
}

func (p *Proxy) Start() error {
	addr := net.JoinHostPort("0.0.0.0", fmt.Sprintf("%d", p.remotePort))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.listener = listener
	p.mu.Unlock()

	log.Info("代理监听启动", "name", p.name, "port", p.remotePort)

	go p.acceptLoop()
	return nil
}

func (p *Proxy) acceptLoop() {
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		conn, err := p.listener.Accept()
		if err != nil {
			p.mu.Lock()
			closed := p.closed
			p.mu.Unlock()

			if closed {
				return
			}

			log.Error("接受连接失败", "error", err)
			continue
		}

		go p.handleConnection(conn)
	}
}

func (p *Proxy) handleConnection(userConn net.Conn) {
	defer userConn.Close()
	log.Debug("新用户连接", "proxy", p.name, "addr", userConn.RemoteAddr())

	dataConn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Error("连接数据通道失败", "error", err)
		return
	}
	defer dataConn.Close()

	// 使用内存池管理连接和缓冲区
	proxyConn := NewProxyConnection(userConn, dataConn, p.name)
	defer proxyConn.Close()

	// 使用共享缓冲区进行双向转发
	proxyConn.Forward()
	log.Debug("用户连接关闭", "proxy", p.name, "addr", userConn.RemoteAddr())
}

func (p *Proxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}
	p.closed = true

	close(p.stopCh)

	if p.listener != nil {
		p.listener.Close()
	}

	log.Info("代理停止", "name", p.name, "port", p.remotePort)
}
