package connect

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	// ErrPoolClosed 连接池已关闭
	ErrPoolClosed = errors.New("connection pool is closed")
)

// PoolConfig 连接池配置
type PoolConfig struct {
	MaxIdle     int           // 最大空闲连接数
	MaxActive   int           // 最大活跃连接数
	IdleTimeout time.Duration // 空闲超时时间
	WaitTimeout time.Duration // 等待获取连接的超时时间
}

// DefaultPoolConfig 默认连接池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxIdle:     5,
		MaxActive:   20,
		IdleTimeout: 60 * time.Second,
		WaitTimeout: 5 * time.Second,
	}
}

// PooledConnection 池化连接
type PooledConnection struct {
	conn       net.Conn
	pool       *ConnectionPool
	createdAt  time.Time
	lastUsedAt time.Time
}

// ConnectionPool 连接池
type ConnectionPool struct {
	config    *PoolConfig
	addr      string
	mu        sync.Mutex
	conns     []*PooledConnection
	numActive int
	closed    bool
	factory   func() (net.Conn, error)
}

// NewConnectionPool 创建连接池
func NewConnectionPool(addr string, config *PoolConfig) *ConnectionPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	pool := &ConnectionPool{
		config: config,
		addr:   addr,
		conns:  make([]*PooledConnection, 0),
		factory: func() (net.Conn, error) {
			return net.DialTimeout("tcp", addr, 5*time.Second)
		},
	}

	return pool
}

// Get 获取连接
func (p *ConnectionPool) Get() (*PooledConnection, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	// 检查活跃连接数
	if p.numActive >= p.config.MaxActive {
		p.mu.Unlock()
		return nil, errors.New("connection pool: too many active connections")
	}

	// 从池中获取空闲连接
	if len(p.conns) > 0 {
		conn := p.conns[len(p.conns)-1]
		p.conns = p.conns[:len(p.conns)-1]

		// 检查连接是否超时
		if time.Since(conn.lastUsedAt) > p.config.IdleTimeout {
			conn.Close()
			p.mu.Unlock()
			return p.createNew()
		}

		p.numActive++
		p.mu.Unlock()
		return conn, nil
	}

	p.mu.Unlock()
	return p.createNew()
}

// createNew 创建新连接
func (p *ConnectionPool) createNew() (*PooledConnection, error) {
	conn, err := p.factory()
	if err != nil {
		return nil, err
	}

	pooled := &PooledConnection{
		conn:       conn,
		pool:       p,
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
	}

	p.mu.Lock()
	p.numActive++
	p.mu.Unlock()

	return pooled, nil
}

// Put 归还连接
func (p *ConnectionPool) Put(conn *PooledConnection) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		conn.Close()
		p.numActive--
		return ErrPoolClosed
	}

	// 更新最后使用时间
	conn.lastUsedAt = time.Now()

	// 如果空闲连接数超过最大值，关闭连接
	if len(p.conns) >= p.config.MaxIdle {
		conn.Close()
		p.numActive--
		return nil
	}

	p.conns = append(p.conns, conn)
	p.numActive--
	return nil
}

// Close 关闭连接池
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// 关闭所有连接
	for _, conn := range p.conns {
		conn.Close()
	}
	p.conns = nil

	return nil
}

// Stats 获取连接池统计信息
func (p *ConnectionPool) Stats() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.conns), p.numActive
}

// Conn 获取底层连接
func (pc *PooledConnection) Conn() net.Conn {
	return pc.conn
}

// Close 关闭连接（不归还到池中）
func (pc *PooledConnection) Close() error {
	if pc.conn == nil {
		return nil
	}
	return pc.conn.Close()
}

// Release 归还连接到池中
func (pc *PooledConnection) Release() error {
	if pc.pool == nil {
		return pc.Close()
	}

	// 检查连接是否仍然可用
	if pc.conn == nil {
		return nil
	}

	return pc.pool.Put(pc)
}
