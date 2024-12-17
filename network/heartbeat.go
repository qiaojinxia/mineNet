package network

import (
	"github.com/prometheus/client_golang/prometheus"
	"mineNet/metrics"
	"sync/atomic"
	"time"
)

type HeartbeatManager struct {
	interval   time.Duration
	timeout    time.Duration
	retryCount int
	conn       IConnection
	lastPing   int64
	lastPong   int64
	stopped    chan struct{}
	metrics    *metrics.HeartbeatMetrics
	labels     prometheus.Labels
}

type HeartPacket struct {
	Seq       uint64 // 序列号
	Timestamp int64  // 发送时间戳
	Type      uint8  // 1=ping 2=pong
}

func NewHeartbeatManager(conn IConnection, interval, timeout time.Duration,
	retryCount int, namespace string) *HeartbeatManager {
	labels := prometheus.Labels{
		"connection_id": conn.ID(),
		"remote_addr":   conn.RemoteAddr(),
	}
	return &HeartbeatManager{
		interval:   interval,
		timeout:    timeout,
		retryCount: retryCount,
		conn:       conn,
		stopped:    make(chan struct{}),
		metrics:    metrics.GetHeartbeatMetrics(namespace),
		labels:     labels,
	}
}

func (h *HeartbeatManager) Start() {
	h.metrics.SetConnected(1, h.labels)
	go h.pingLoop()
	go h.checkLoop()
}

func (h *HeartbeatManager) Stop() {
	close(h.stopped)
	h.metrics.SetConnected(0, h.labels)
	h.metrics.Reset(h.labels)
}

func (h *HeartbeatManager) pingLoop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopped:
			return
		case <-ticker.C:
			ping := &HeartPacket{Timestamp: time.Now().UnixNano()}
			data, _ := NewHeartPacket(ping)
			err := h.conn.Send(data)
			if err != nil {
				h.metrics.IncFailed(h.labels)
				return
			}
			atomic.StoreInt64(&h.lastPing, ping.Timestamp)
		}
	}
}

func (h *HeartbeatManager) checkLoop() {
	ticker := time.NewTicker(h.timeout)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopped:
			return
		case <-ticker.C:
			if !h.IsAlive() {
				h.metrics.IncTimeouts(h.labels)
				h.handleTimeout()
			}
		}
	}
}

func (h *HeartbeatManager) IsAlive() bool {
	lastPong := atomic.LoadInt64(&h.lastPong)
	if lastPong == 0 {
		return true // 初始状态
	}
	return time.Now().UnixNano()-lastPong < h.timeout.Nanoseconds()
}

func (h *HeartbeatManager) handleTimeout() {
	h.metrics.SetConnected(0, h.labels)
	for i := 0; i < h.retryCount; i++ {
		h.metrics.IncReconnects(h.labels)
		if h.reconnect() {
			h.metrics.SetConnected(1, h.labels)
			return
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}
	// 重连失败，关闭连接
	err := h.conn.Close()
	if err != nil {
		return
	}
}

func (h *HeartbeatManager) reconnect() bool {
	// 实现重连逻辑
	return false
}

// HandlePong 处理pong响应
func (h *HeartbeatManager) HandlePong(pong *HeartPacket) {
	now := time.Now()
	atomic.StoreInt64(&h.lastPong, now.UnixNano())

	// 计算RTT
	rtt := time.Duration(now.UnixNano() - pong.Timestamp)
	h.metrics.ObserveRtt(rtt.Seconds(), h.labels)
	h.metrics.IncSuccess(h.labels)
	h.metrics.SetLastSuccess(float64(now.Unix()), h.labels)
}
