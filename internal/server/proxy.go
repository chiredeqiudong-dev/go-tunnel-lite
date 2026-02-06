package server

import (
	"fmt"
	"net"
	"sync"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proxy"
)

type Proxy struct {
	name       string
	remotePort int
	listener   net.Listener
	stopCh     chan struct{}
	mu         sync.Mutex
	closed     bool
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

	// 使用共享的代理连接
	proxyConn := proxy.NewProxyConnection(userConn, dataConn, p.name)
	defer proxyConn.Close()

	// 使用零拷贝进行双向转发
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
