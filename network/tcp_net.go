package network

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"mineNet/common"
	mconfig "mineNet/config"
	"mineNet/metrics"
	"net"
	"strings"
	"sync"
	"time"
)

// TCPServer TCP服务器
type TCPServer struct {
	config     *mconfig.ServerConfig
	listener   net.Listener
	metrics    *metrics.TcpServerMetrics
	conns      *ConnPool
	connConfig *mconfig.ConnConfig
	ctx        context.Context
	cancel     context.CancelFunc
	startTime  time.Time

	onConnect     func(IConnection)
	onDisconnect  func(IConnection)
	handlers      map[uint32]PacketHandler
	handlersMutex sync.RWMutex
}

func (s *TCPServer) GetConnections() []IConnection {
	defer func() {
		if r := recover(); r != nil {
			common.GetLogger().Error("panic in GetConnections",
				zap.Any("recover", r),
				zap.Stack("stack"),
			)
		}
	}()

	connections := s.conns.GetAll()
	result := make([]IConnection, len(connections))
	for i, conn := range connections {
		result[i] = conn
	}
	return result
}

func InitTcpServer(serverCfg *mconfig.Config) (*TCPServer, error) {

	ctx, cancel := context.WithCancel(context.Background())
	connPool := NewConnPool(&serverCfg.ConnPool)

	s := &TCPServer{
		config:     &serverCfg.Server,
		metrics:    metrics.GetTcpServerMetrics("tcp_server"),
		conns:      connPool,
		connConfig: &serverCfg.Conn,
		ctx:        ctx,
		cancel:     cancel,
		handlers:   make(map[uint32]PacketHandler),
	}

	go s.collectMetrics()

	return s, nil
}

func (s *TCPServer) Start() error {
	listener, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		s.metrics.StartupErrors.Inc()
		common.GetLogger().Error("failed to start server",
			zap.String("addr", s.config.Addr),
			zap.Error(err),
		)
		return fmt.Errorf("failed to start server: %w", err)
	}
	s.listener = listener

	common.GetLogger().Info("server started",
		zap.String("addr", s.config.Addr),
	)

	s.metrics.ServerStatus.Set(1)
	s.metrics.StartupTime.SetToCurrentTime()
	s.startTime = time.Now()
	s.acceptLoop()

	return nil
}

func (s *TCPServer) Stop() error {
	s.cancel()
	startTime := time.Now()

	if err := s.conns.Close(); err != nil {
		s.metrics.ShutdownErrors.Inc()
		common.GetLogger().Error("failed closing connections",
			zap.Error(err),
		)
	}

	if s.listener != nil {
		// 等待现有连接处理完成
		if err := s.listener.Close(); err != nil {
			s.metrics.ShutdownErrors.Inc()
			common.GetLogger().Error("error closing listener",
				zap.Error(err),
			)
			return fmt.Errorf("failed to close listener: %w", err)
		}
	}

	common.GetLogger().Info("server stopped",
		zap.Duration("shutdown_duration", time.Since(startTime)),
	)

	s.metrics.ServerStatus.Set(0)
	s.metrics.ShutdownDuration.Observe(time.Since(startTime).Seconds())
	return nil
}

func (s *TCPServer) acceptLoop() {
	defer func() {
		if r := recover(); r != nil {
			common.GetLogger().Error("panic in acceptLoop",
				zap.Any("recover", r),
				zap.Stack("stack"),
			)
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			common.GetLogger().Info("acceptLoop stopped due to context cancellation")
			return
		default:
			startTime := time.Now()
			conn, err := s.listener.Accept()
			// 处理Accept错误
			if err != nil {
				// 检查是否是由于listener被关闭导致的错误
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Temporary() {
					// 临时错误，等待后重试
					common.GetLogger().Warn("temporary accept error, retrying",
						zap.Error(err),
						zap.Duration("retry_after", time.Second),
					)
					time.Sleep(time.Second)
					continue
				}

				// 检查是否是由于关闭导致的错误
				if strings.Contains(err.Error(), "use of closed network connection") {
					common.GetLogger().Info("listener closed, stopping accept loop")
					return
				}

				// 其他错误
				s.metrics.AcceptErrors.Inc()
				common.GetLogger().Error("accept error",
					zap.Error(err),
				)
				continue
			}

			s.metrics.AcceptTotal.Inc()
			s.metrics.AcceptDuration.Observe(time.Since(startTime).Seconds())

			c := NewConnection(conn, s.connConfig)

			s.handlersMutex.RLock()
			for cmdId, handler := range s.handlers {
				c.SetHandler(cmdId, handler)
			}
			s.handlersMutex.RUnlock()

			if err := s.conns.Add(c); err != nil {
				s.metrics.AcceptErrors.Inc()
				common.GetLogger().Error("failed to add connection",
					zap.String("remote_addr", conn.RemoteAddr().String()),
					zap.Error(err),
				)
				if err := c.Close(); err != nil {
					common.GetLogger().Error("failed to close connection",
						zap.String("remote_addr", conn.RemoteAddr().String()),
						zap.Error(err),
					)
				}
				continue
			}

			common.GetLogger().Info("new connection accepted",
				zap.String("conn_id", c.ID()),
				zap.String("remote_addr", conn.RemoteAddr().String()),
			)

			s.metrics.ConnectionsCreatedTotal.Inc()

			if s.onConnect != nil {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							common.GetLogger().Error("panic in onConnect callback",
								zap.String("conn_id", c.ID()),
								zap.Any("recover", r),
								zap.Stack("stack"),
							)
						}
					}()
					s.onConnect(c)
				}()
			}

			go s.monitorConnection(c)
		}
	}
}

func (s *TCPServer) monitorConnection(conn *Connection) {
	defer func() {
		if r := recover(); r != nil {
			common.GetLogger().Error("panic in monitorConnection",
				zap.String("conn_id", conn.ID()),
				zap.Any("recover", r),
				zap.Stack("stack"),
			)
		}
	}()

	connStartTime := time.Now()

	<-conn.closeChan

	if err := s.conns.Remove(conn.ID()); err != nil {
		common.GetLogger().Error("failed to remove connection",
			zap.String("conn_id", conn.ID()),
			zap.Error(err),
		)
	}

	if s.onDisconnect != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					common.GetLogger().Error("panic in onDisconnect callback",
						zap.String("conn_id", conn.ID()),
						zap.Any("recover", r),
						zap.Stack("stack"),
					)
				}
			}()
			s.onDisconnect(conn)
		}()
	}

	common.GetLogger().Info("connection closed",
		zap.String("conn_id", conn.ID()),
		zap.Duration("lifetime", time.Since(connStartTime)),
	)

	s.metrics.ConnectionsClosedTotal.Inc()
	s.metrics.ConnectionLifetime.Observe(time.Since(connStartTime).Seconds())
}

func (s *TCPServer) Broadcast(packet IPacket) {
	startTime := time.Now()
	successCount := 0
	totalConns := 0

	for _, conn := range s.conns.GetAll() {
		if !conn.IsClosed() {
			totalConns++
			if err := conn.Send(packet); err == nil {
				successCount++
			} else {
				s.metrics.BroadcastErrors.Inc()
				common.GetLogger().Error("broadcast error",
					zap.String("conn_id", conn.ID()),
					zap.Error(err),
				)
			}
		}
	}

	s.metrics.BroadcastTotal.Inc()
	s.metrics.BroadcastDuration.Observe(time.Since(startTime).Seconds())

	if totalConns > 0 {
		s.metrics.BroadcastSuccessRate.Set(float64(successCount) / float64(totalConns))
	}

	common.GetLogger().Debug("broadcast completed",
		zap.Int("total_conns", totalConns),
		zap.Int("success_count", successCount),
		zap.Duration("duration", time.Since(startTime)),
	)
}

func (s *TCPServer) OnConnect(fn func(IConnection)) {
	s.onConnect = fn
}

func (s *TCPServer) OnDisconnect(fn func(IConnection)) {
	s.onDisconnect = fn
}

func (s *TCPServer) SetHandler(cmdId uint32, handler PacketHandler) {
	s.handlersMutex.Lock()
	defer s.handlersMutex.Unlock()
	s.handlers[cmdId] = handler
	common.GetLogger().Debug("handler set",
		zap.Uint32("cmd_id", cmdId),
	)
}

func (s *TCPServer) getHandler(cmdId uint32) (PacketHandler, bool) {
	s.handlersMutex.RLock()
	defer s.handlersMutex.RUnlock()
	handler, ok := s.handlers[cmdId]
	return handler, ok
}

func (s *TCPServer) GetActiveConnections() int {
	defer func() {
		if r := recover(); r != nil {
			common.GetLogger().Error("panic in GetActiveConnections",
				zap.Any("recover", r),
				zap.Stack("stack"),
			)
		}
	}()

	activeCount := 0
	for _, conn := range s.conns.GetAll() {
		if !conn.IsClosed() {
			activeCount++
		}
	}
	return activeCount
}

// collectMetrics 定期收集服务器指标
func (s *TCPServer) collectMetrics() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 获取连接池状态
			poolStatus := s.conns.Status()
			activeConns := poolStatus["total_connections"].(int)

			// 更新指标
			s.metrics.ActiveConnectionGauge.Set(float64(activeConns))
			s.metrics.UptimeSeconds.Set(time.Since(s.startTime).Seconds())

			// 计算连接使用率
			if s.config.MaxConnections > 0 {
				utilizationRate := float64(activeConns) / float64(s.config.MaxConnections)
				s.metrics.ConnectionUtilization.Set(utilizationRate)
			}
		}
	}
}
