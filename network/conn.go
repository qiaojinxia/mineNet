package network

import (
	"context"
	"encoding/json"
	"errors"
	"go.uber.org/zap"
	"io"
	"mineNet/common"
	mconfig "mineNet/config"
	"mineNet/metrics"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	HeartbeatPing uint8 = 1
	HeartbeatPong uint8 = 2
)

var ErrConnectionClosed = errors.New("connection is closed")
var ErrRateLimitExceeded = errors.New("rate limit exceeded")
var ErrSendQueueFull = errors.New("send queue is full")

// Connection 连接的信息
type Connection struct {
	id        string
	conn      net.Conn
	codec     *PacketCodec
	heartbeat *HeartbeatManager
	limiter   *RateLimiter
	metrics   *metrics.ConnMetrics
	sendChan  chan *Packet
	recvChan  chan *Packet
	closeChan chan struct{}

	properties sync.Map

	createdAt    time.Time    // 创建时间
	lastActive   int64        // 最后活跃时间
	errorCount   atomic.Int32 // 错误计数
	closed       atomic.Bool  // 关闭状态
	labelValues  []string     //统计label
	heartbeatSeq uint64       // 心跳序列号
	messageSeq   uint64       // 消息序列号

	handlers      map[uint32]PacketHandler // 该连接的消息处理器
	handlersMutex sync.RWMutex             // 处理器映射锁
}

// NewConnection 构造函数
func NewConnection(conn net.Conn, config *mconfig.ConnConfig) *Connection {
	now := time.Now()
	c := &Connection{
		id:   common.GenerateConnectionId(),
		conn: conn,
		codec: NewPacketCodec(config.CompressEnabled, config.EncryptEnabled,
			[]byte(config.EncryptKey), []byte(config.EncryptIV)),
		metrics:       metrics.GetConnMetrics("connection"),
		sendChan:      make(chan *Packet, 1024),
		recvChan:      make(chan *Packet, 1024),
		closeChan:     make(chan struct{}),
		createdAt:     now,
		handlers:      make(map[uint32]PacketHandler),
		handlersMutex: sync.RWMutex{},
	}
	common.GetLogger().Info("creating new connection",
		zap.String("remote_addr", conn.RemoteAddr().String()),
		zap.String("local_addr", conn.LocalAddr().String()),
	)
	c.limiter = GetRateLimiter(config.RPS, config.BurstSize)
	c.heartbeat = NewHeartbeatManager(c, config.HeartbeatInterval,
		config.HeartbeatTimeout, config.RetryCount, "heartbeat")
	c.lastActive = now.UnixNano()
	c.closed.Store(false)
	c.labelValues = []string{
		c.id,                       // conn_id
		conn.RemoteAddr().String(), // remote_addr
		conn.LocalAddr().String(),  // local_addr
	}
	c.metrics.Connected.WithLabelValues(c.labelValues...).Set(1)
	go c.readLoop()
	go c.writeLoop()
	c.heartbeat.Start()

	return c
}

// ID 返回连接唯一标识
func (c *Connection) ID() string {
	return c.id
}

// Send 发送消息
func (c *Connection) Send(packet IPacket) error {
	labelValues := []string{c.id, c.RemoteAddr(), c.conn.LocalAddr().String()}

	if c.IsClosed() {
		return ErrConnectionClosed
	}

	c.updateLastActive()

	if !c.limiter.Allow() {
		c.metrics.MsgDropTotal.WithLabelValues(labelValues...).Inc()
		return ErrRateLimitExceeded
	}

	newSeq := atomic.AddUint64(&c.messageSeq, 1)
	packet.SetSeqId(newSeq)

	select {
	case c.sendChan <- packet.(*Packet):
		c.metrics.MsgOutTotal.WithLabelValues(labelValues...).Inc()
		return nil
	case <-c.closeChan:
		return ErrConnectionClosed
	default:
		c.metrics.MsgDropTotal.WithLabelValues(labelValues...).Inc()
		return ErrSendQueueFull
	}
}

// Close 关闭连接
func (c *Connection) Close() error {

	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(c.closeChan)
	c.heartbeat.Stop()
	c.metrics.Connected.WithLabelValues(c.labelValues...).Set(0)

	return c.conn.Close()
}

// RemoteAddr 获取远程地址
func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

// GetProperty 获取连接属性
func (c *Connection) GetProperty(key string) any {
	if value, ok := c.properties.Load(key); ok {
		return value
	}
	return nil
}

// SetProperty 设置连接属性
func (c *Connection) SetProperty(key string, value any) {
	c.properties.Store(key, value)
}

// LastActive 获取最后活跃时间
func (c *Connection) LastActive() time.Time {
	return time.Unix(0, atomic.LoadInt64(&c.lastActive))
}

// CreatedAt 获取创建时间
func (c *Connection) CreatedAt() time.Time {
	return c.createdAt
}

// ErrorCount 获取错误次数
func (c *Connection) ErrorCount() int {
	return int(c.errorCount.Load())
}

// IsClosed 连接是否已关闭
func (c *Connection) IsClosed() bool {
	return c.closed.Load()
}

// IsAlive 判断是否超时未响应
func (c *Connection) IsAlive() bool {
	return c.heartbeat.IsAlive()
}

// SetHandler 设置消息处理器
func (c *Connection) SetHandler(cmdId uint32, handler PacketHandler) {
	c.handlers[cmdId] = handler
}

// 内部辅助方法

// incrementErrorCount 增加错误计数
func (c *Connection) incrementErrorCount() {
	c.errorCount.Add(1)
}

func (c *Connection) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			common.GetLogger().Error("panic in readLoop",
				zap.String("conn_id", c.id),
				zap.String("remote_addr", c.RemoteAddr()),
				zap.Any("recover", r),
				zap.Stack("stack"),
			)
		}
		_ = c.Close()
	}()

	for {
		select {
		case <-c.closeChan:
			return
		default:
			packet, err := c.codec.Decode(c.conn)
			if err == io.EOF {
				common.GetLogger().Info("connection closed by peer",
					zap.String("conn_id", c.id),
					zap.String("remote_addr", c.RemoteAddr()),
				)
				return
			}
			// 处理超时错误
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				common.GetLogger().Warn("connection read timeout",
					zap.String("conn_id", c.id),
					zap.String("remote_addr", c.RemoteAddr()),
					zap.Error(err),
				)
				c.metrics.TimeoutTotal.WithLabelValues(c.labelValues...).Inc()
				continue
			}

			if err != nil {
				common.GetLogger().Error("decode error",
					zap.String("conn_id", c.id),
					zap.String("remote_addr", c.RemoteAddr()),
					zap.Error(err),
				)
				c.metrics.RecvErrors.WithLabelValues(c.labelValues...).Inc()
				c.metrics.ErrorTotal.WithLabelValues(c.labelValues...).Inc()
				c.incrementErrorCount()
				return
			}

			c.updateLastActive()
			c.metrics.BytesInTotal.WithLabelValues(c.labelValues...).Add(float64(len(packet.Data())))
			c.metrics.MsgInTotal.WithLabelValues(c.labelValues...).Inc()
			c.metrics.MsgTotal.WithLabelValues(c.labelValues...).Inc()

			if packet.CmdId() == CmdPing {
				if err := c.handleHeartbeat(packet); err != nil {
					common.GetLogger().Error("handle heartbeat error",
						zap.String("conn_id", c.id),
						zap.String("remote_addr", c.RemoteAddr()),
						zap.Error(err),
					)
					continue
				}
				continue
			} else {
				// 使用连接自己的处理器处理消息
				c.handlersMutex.RLock()
				handler, ok := c.handlers[packet.CmdId()]
				c.handlersMutex.RUnlock()
				if ok {
					go func() {
						defer func() {
							if r := recover(); r != nil {
								common.GetLogger().Error("panic in packet handler",
									zap.String("conn_id", c.id),
									zap.String("remote_addr", c.RemoteAddr()),
									zap.Uint32("cmd_id", packet.CmdId()),
									zap.Any("recover", r),
									zap.Stack("stack"),
								)
							}
						}()

						if err := handler.Handle(context.Background(), c, packet); err != nil {
							common.GetLogger().Error("handle packet error",
								zap.String("conn_id", c.id),
								zap.String("remote_addr", c.RemoteAddr()),
								zap.Uint32("cmd_id", packet.CmdId()),
								zap.Error(err),
							)
						}
					}()
					continue
				} else {
					// 未找到对应的协议处理器
					common.GetLogger().Error("handler not found for packet",
						zap.String("conn_id", c.id),
						zap.String("remote_addr", c.RemoteAddr()),
						zap.Uint32("cmd_id", packet.CmdId()),
					)
					c.metrics.ErrorTotal.WithLabelValues(c.labelValues...).Inc()
					continue
				}
			}

		}
	}
}

func (c *Connection) handleHeartbeat(msg *Packet) error {
	// 更新最后活跃时间
	c.updateLastActive()
	data := HeartPacket{}
	err := json.Unmarshal(msg.Data(), &data)
	if err != nil {
		return err
	}
	switch data.Type {
	case HeartbeatPing:
		// 收到ping请求，直接返回pong
		var ping HeartPacket
		if err := json.Unmarshal(msg.Data(), &ping); err != nil {
			c.heartbeat.metrics.IncFailed(c.heartbeat.labels)
			return err
		}
		newSeq := atomic.AddUint64(&c.heartbeatSeq, 1)
		hp := &HeartPacket{
			Seq:       newSeq,
			Timestamp: time.Now().UnixNano(),
			Type:      HeartbeatPong,
		}
		pong, err := NewHeartPacket(hp)
		if err != nil {
			c.heartbeat.metrics.IncFailed(c.heartbeat.labels)
			return err
		}
		// 发送pong响应
		if err := c.Send(pong); err != nil {
			c.heartbeat.metrics.IncFailed(c.heartbeat.labels)
			return err
		}
	case HeartbeatPong:
		// 收到pong响应
		var pong HeartPacket
		if err := json.Unmarshal(msg.Data(), &pong); err != nil {
			c.heartbeat.metrics.IncFailed(c.heartbeat.labels)
			return err
		}
		// 处理pong响应
		c.heartbeat.HandlePong(&pong)
	}

	return nil
}

// UpdateLastActive 更新最后活跃时间
func (c *Connection) updateLastActive() {
	now := time.Now()
	// 原子操作更新最后活跃时间
	last := time.Unix(0, atomic.SwapInt64(&c.lastActive, now.UnixNano()))

	// 计算空闲时间并记录到metrics
	if !last.IsZero() {
		idleTime := now.Sub(last)
		c.metrics.IdleTime.WithLabelValues(c.labelValues...).Observe(idleTime.Seconds())
	}

	// 更新活跃状态
	c.metrics.Active.WithLabelValues(c.labelValues...).Set(1)
}

func (c *Connection) writeLoop() {
	defer func() {
		if r := recover(); r != nil {
			common.GetLogger().Error("panic in writeLoop",
				zap.String("conn_id", c.id),
				zap.String("remote_addr", c.RemoteAddr()),
				zap.Any("recover", r),
				zap.Stack("stack"),
			)
		}
		_ = c.Close()
	}()

	for {
		select {
		case msg := <-c.sendChan:
			start := time.Now()

			data, err := c.codec.Encode(msg)
			if err != nil {
				common.GetLogger().Error("encode error",
					zap.String("conn_id", c.id),
					zap.String("remote_addr", c.RemoteAddr()),
					zap.Error(err),
				)
				c.metrics.CodecErrors.WithLabelValues(c.labelValues...).Inc()
				c.metrics.ErrorTotal.WithLabelValues(c.labelValues...).Inc()
				c.incrementErrorCount()
				continue
			}

			if _, err := c.conn.Write(data); err != nil {
				common.GetLogger().Error("write error",
					zap.String("conn_id", c.id),
					zap.String("remote_addr", c.RemoteAddr()),
					zap.Error(err),
				)
				c.metrics.SendErrors.WithLabelValues(c.labelValues...).Inc()
				c.metrics.ErrorTotal.WithLabelValues(c.labelValues...).Inc()
				c.incrementErrorCount()
				return
			}

			c.metrics.BytesOutTotal.WithLabelValues(c.labelValues...).Add(float64(len(data)))
			c.metrics.LatencyHist.WithLabelValues(c.labelValues...).Observe(time.Since(start).Seconds())

		case <-c.closeChan:
			return
		}
	}
}
