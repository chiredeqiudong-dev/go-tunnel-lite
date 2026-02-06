package proxy

import (
	"io"
	"net"
	"sync"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
)

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

// ProxyConnection 代理连接，使用零拷贝
type ProxyConnection struct {
	localConn  net.Conn
	remoteConn net.Conn
	proxyID    string
}

// NewProxyConnection 创建代理连接
func NewProxyConnection(local, remote net.Conn, proxyID string) *ProxyConnection {
	return &ProxyConnection{
		localConn:  local,
		remoteConn: remote,
		proxyID:    proxyID,
	}
}

// Close 关闭代理连接
func (pc *ProxyConnection) Close() {
	if pc.localConn != nil {
		pc.localConn.Close()
	}
	if pc.remoteConn != nil {
		pc.remoteConn.Close()
	}
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
