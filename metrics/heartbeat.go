package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var (
	HeartInstance *HeartbeatMetrics
	HeartOnce     sync.Once
)

// HeartbeatMetrics 心跳相关的监控指标
type HeartbeatMetrics struct {
	// 连接状态 (0/1)
	Connected *prometheus.GaugeVec
	// 心跳成功次数
	Success *prometheus.CounterVec
	// 心跳失败次数
	Failed *prometheus.CounterVec
	// 超时次数
	Timeouts *prometheus.CounterVec
	// 重连次数
	Reconnects *prometheus.CounterVec
	// RTT延迟分布
	Rtt *prometheus.HistogramVec
	// 最后一次成功的心跳时间
	LastSuccess *prometheus.GaugeVec
}

// GetHeartbeatMetrics 获取单例实例
func GetHeartbeatMetrics(namespace string) *HeartbeatMetrics {
	HeartOnce.Do(func() {
		HeartInstance = newHeartbeatMetrics(namespace)
	})
	return HeartInstance
}

// newHeartbeatMetrics 创建新的心跳指标集合
func newHeartbeatMetrics(namespace string) *HeartbeatMetrics {
	labelNames := []string{"connection_id", "remote_addr"}

	metrics := &HeartbeatMetrics{
		Connected: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "heartbeat",
				Name:      "connected",
				Help:      "Connection status (0=disconnected, 1=connected)",
			},
			labelNames,
		),

		Success: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "heartbeat",
				Name:      "success_total",
				Help:      "Total number of successful heartbeats",
			},
			labelNames,
		),

		Failed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "heartbeat",
				Name:      "failed_total",
				Help:      "Total number of failed heartbeats",
			},
			labelNames,
		),

		Timeouts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "heartbeat",
				Name:      "timeouts_total",
				Help:      "Total number of heartbeat timeouts",
			},
			labelNames,
		),

		Reconnects: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "heartbeat",
				Name:      "reconnects_total",
				Help:      "Total number of reconnection attempts",
			},
			labelNames,
		),

		Rtt: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "heartbeat",
				Name:      "rtt_seconds",
				Help:      "Round-trip time in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			labelNames,
		),

		LastSuccess: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "heartbeat",
				Name:      "last_success_timestamp_seconds",
				Help:      "Timestamp of the last successful heartbeat",
			},
			labelNames,
		),
	}

	// 注册所有指标
	prometheus.MustRegister(
		metrics.Connected,
		metrics.Success,
		metrics.Failed,
		metrics.Timeouts,
		metrics.Reconnects,
		metrics.Rtt,
		metrics.LastSuccess,
	)

	return metrics
}

// Reset 重置所有计数器
func (m *HeartbeatMetrics) Reset(labels prometheus.Labels) {
	m.Connected.With(labels).Set(0)
	// Counter类型的指标不能重置
	m.LastSuccess.With(labels).Set(0)
}

// 为了方便使用，添加一些辅助方法

func (m *HeartbeatMetrics) SetConnected(v float64, labels prometheus.Labels) {
	m.Connected.With(labels).Set(v)
}

func (m *HeartbeatMetrics) IncSuccess(labels prometheus.Labels) {
	m.Success.With(labels).Inc()
}

func (m *HeartbeatMetrics) IncFailed(labels prometheus.Labels) {
	m.Failed.With(labels).Inc()
}

func (m *HeartbeatMetrics) IncTimeouts(labels prometheus.Labels) {
	m.Timeouts.With(labels).Inc()
}

func (m *HeartbeatMetrics) IncReconnects(labels prometheus.Labels) {
	m.Reconnects.With(labels).Inc()
}

func (m *HeartbeatMetrics) ObserveRtt(v float64, labels prometheus.Labels) {
	m.Rtt.With(labels).Observe(v)
}

func (m *HeartbeatMetrics) SetLastSuccess(v float64, labels prometheus.Labels) {
	m.LastSuccess.With(labels).Set(v)
}
