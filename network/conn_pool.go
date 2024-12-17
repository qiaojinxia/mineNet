package network

import (
	"fmt"
	mconfig "mineNet/config"
	"mineNet/metrics"
	"sync"
	"time"
)

var (
	ErrPoolFull   = fmt.Errorf("pool full")
	ErrConnClosed = fmt.Errorf("connection closed")
)

type ConnPool struct {
	sync.RWMutex
	connections   map[string]IConnection
	maxConns      int
	maxErrorCount int
	minIdleConns  int
	idleTimeout   time.Duration
	maxLifetime   time.Duration
	maxIdleTime   time.Duration
	metrics       *metrics.ConnPoolMetrics
}

func NewConnPool(config *mconfig.ConnPoolConfig) *ConnPool {
	pool := &ConnPool{
		connections:   make(map[string]IConnection),
		maxConns:      config.MaxConns,
		minIdleConns:  config.MinIdleConns,
		maxErrorCount: config.MaxErrorCount,
		idleTimeout:   config.IdleTimeout,
		maxLifetime:   config.MaxLifetime,
		maxIdleTime:   config.MaxIdleTime,
		metrics:       metrics.GetConnPoolMetrics(),
	}

	// 启动清理协程
	go pool.cleanup()
	return pool
}

// Add 添加新连接到连接池
func (p *ConnPool) Add(conn IConnection) error {
	startTime := time.Now()
	p.Lock()
	defer p.Unlock()

	if len(p.connections) >= p.maxConns {
		p.metrics.ConnAcquireFailed.Inc()
		return ErrPoolFull
	}

	p.connections[conn.ID()] = conn

	// 更新指标
	p.metrics.TotalConns.Inc()
	p.metrics.ConnCreated.Inc()
	p.metrics.AcquireTime.Observe(time.Since(startTime).Seconds())
	p.metrics.ConnAcquired.Inc()

	return nil
}

// Remove 从连接池移除连接
func (p *ConnPool) Remove(connID string) error {
	p.Lock()
	defer p.Unlock()

	if conn, exists := p.connections[connID]; exists {
		err := conn.Close()
		if err != nil {
			p.metrics.CloseErrors.Inc()
			return err
		}
		delete(p.connections, connID)

		// 更新指标
		p.metrics.TotalConns.Dec()
		p.metrics.ConnClosed.Inc()
	}
	return nil
}

// Get 获取一个连接
func (p *ConnPool) Get(id string) (IConnection, error) {
	p.RLock()
	defer p.RUnlock()

	if conn, exists := p.connections[id]; exists {
		if conn.IsClosed() {
			return nil, ErrConnClosed
		}
		p.metrics.ConnAcquired.Inc()
		return conn, nil
	}
	return nil, fmt.Errorf("connection not found: %s", id)
}

// GetAll 获取所有连接
func (p *ConnPool) GetAll() []IConnection {
	p.RLock()
	defer p.RUnlock()

	conns := make([]IConnection, 0, len(p.connections))
	for _, conn := range p.connections {
		conns = append(conns, conn)
	}
	return conns
}

// Len 返回当前连接数
func (p *ConnPool) Len() int {
	p.RLock()
	defer p.RUnlock()
	return len(p.connections)
}

func (p *ConnPool) shouldCleanup(conn IConnection) bool {
	// 1. 检查连接是否已关闭
	if conn.IsClosed() {
		return true
	}

	// 2. 检查空闲时间
	idleTime := time.Since(conn.LastActive())
	if idleTime > p.maxIdleTime && len(p.connections) > p.minIdleConns {
		return true
	}

	// 3. 检查存活时间
	lifetime := time.Since(conn.CreatedAt())
	if lifetime > p.maxLifetime {
		return true
	}

	// 4. 检查错误次数
	if conn.ErrorCount() > p.maxErrorCount {
		return true
	}

	// 5. 检查心跳状态
	if !conn.IsAlive() {
		return true
	}

	return false
}

func (p *ConnPool) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.Lock()
		initialCount := len(p.connections)

		for id, conn := range p.connections {
			if p.shouldCleanup(conn) {
				err := conn.Close()
				if err != nil {
					p.metrics.CloseErrors.Inc()
					continue
				}
				delete(p.connections, id)
				p.metrics.TotalConns.Dec()
				p.metrics.ConnClosed.Inc()
			}
		}

		// 更新清理指标
		p.metrics.CleanupTotal.Inc()
		cleaned := initialCount - len(p.connections)
		if cleaned > 0 {
			p.metrics.CleanupConns.Add(float64(cleaned))
		}
		p.Unlock()
	}
}

// Close 关闭连接池
func (p *ConnPool) Close() error {
	p.Lock()
	defer p.Unlock()

	for id, conn := range p.connections {
		err := conn.Close()
		if err != nil {
			p.metrics.CloseErrors.Inc()
			continue
		}
		delete(p.connections, id)
		p.metrics.TotalConns.Dec()
		p.metrics.ConnClosed.Inc()
	}
	return nil
}

// Status 返回连接池状态
func (p *ConnPool) Status() map[string]interface{} {
	p.RLock()
	defer p.RUnlock()

	return map[string]interface{}{
		"total_connections": len(p.connections),
		"max_connections":   p.maxConns,
		"min_idle_conns":    p.minIdleConns,
		"max_error_count":   p.maxErrorCount,
		"idle_timeout":      p.idleTimeout.String(),
		"max_lifetime":      p.maxLifetime.String(),
		"max_idle_time":     p.maxIdleTime.String(),
	}
}
