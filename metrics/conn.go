package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var (
	ConnOnce     sync.Once
	ConnInstance *ConnMetrics
)

type ConnMetrics struct {
	// 消息计数
	MsgTotal     *prometheus.CounterVec // 总消息数
	MsgInTotal   *prometheus.CounterVec // 接收消息数
	MsgOutTotal  *prometheus.CounterVec // 发送消息数
	MsgDropTotal *prometheus.CounterVec // 丢弃消息数

	// 字节计数
	BytesInTotal  *prometheus.CounterVec // 接收字节数
	BytesOutTotal *prometheus.CounterVec // 发送字节数

	// 错误计数
	ErrorTotal  *prometheus.CounterVec // 总错误数
	SendErrors  *prometheus.CounterVec // 发送错误
	RecvErrors  *prometheus.CounterVec // 接收错误
	CodecErrors *prometheus.CounterVec // 编解码错误

	// 队列状态
	SendQueueSize *prometheus.GaugeVec // 发送队列大小
	RecvQueueSize *prometheus.GaugeVec // 接收队列大小

	// 延迟统计
	LatencyHist *prometheus.HistogramVec // 消息延迟分布

	// 连接状态
	Connected     *prometheus.GaugeVec // 是否连接
	ConnectedTime *prometheus.GaugeVec // 连接持续时间

	// 心跳相关
	HeartbeatTotal     *prometheus.CounterVec // 心跳次数
	HeartbeatFailTotal *prometheus.CounterVec // 心跳失败次数
	LastHeartbeat      *prometheus.GaugeVec   // 上次心跳时间

	// 超时相关
	TimeoutTotal *prometheus.CounterVec // 超时总次数
	ReadTimeout  *prometheus.CounterVec // 读超时次数
	WriteTimeout *prometheus.CounterVec // 写超时次数

	// 连接空闲时间
	IdleTime *prometheus.HistogramVec
	// 活跃状态 1=active, 0=idle
	Active *prometheus.GaugeVec
}

// GetConnMetrics 获取单例指标实例
func GetConnMetrics(namespace string) *ConnMetrics {
	ConnOnce.Do(func() {
		ConnInstance = newConnMetrics(namespace)
	})
	return ConnInstance
}

func newConnMetrics(namespace string) *ConnMetrics {
	// 定义标签
	labels := []string{"conn_id", "remote_addr", "local_addr"}

	m := &ConnMetrics{
		MsgTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_total",
				Help:      "Total number of messages",
			},
			labels,
		),

		MsgInTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_received_total",
				Help:      "Total number of received messages",
			},
			labels,
		),

		MsgOutTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_sent_total",
				Help:      "Total number of sent messages",
			},
			labels,
		),

		MsgDropTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_dropped_total",
				Help:      "Total number of dropped messages",
			},
			labels,
		),

		BytesInTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "bytes_received_total",
				Help:      "Total bytes received",
			},
			labels,
		),

		BytesOutTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "bytes_sent_total",
				Help:      "Total bytes sent",
			},
			labels,
		),

		ErrorTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total number of errors",
			},
			labels,
		),

		SendErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "send_errors_total",
				Help:      "Total number of send errors",
			},
			labels,
		),

		RecvErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "receive_errors_total",
				Help:      "Total number of receive errors",
			},
			labels,
		),

		CodecErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "codec_errors_total",
				Help:      "Total number of codec errors",
			},
			labels,
		),

		SendQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "send_queue_size",
				Help:      "Current size of send queue",
			},
			labels,
		),

		RecvQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "receive_queue_size",
				Help:      "Current size of receive queue",
			},
			labels,
		),

		LatencyHist: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "message_latency_seconds",
				Help:      "Message latency in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			labels,
		),

		Connected: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "connected",
				Help:      "Connection status (1 for connected, 0 for disconnected)",
			},
			labels,
		),

		ConnectedTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "connected_seconds",
				Help:      "Time duration since last connected in seconds",
			},
			labels,
		),

		HeartbeatTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "heartbeats_total",
				Help:      "Total number of heartbeats",
			},
			labels,
		),

		HeartbeatFailTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "heartbeat_failures_total",
				Help:      "Total number of heartbeat failures",
			},
			labels,
		),

		LastHeartbeat: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "last_heartbeat_timestamp",
				Help:      "Timestamp of last successful heartbeat",
			},
			labels,
		),
		TimeoutTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "timeouts_total",
				Help:      "Total number of timeouts",
			},
			labels,
		),

		ReadTimeout: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "read_timeouts_total",
				Help:      "Total number of read timeouts",
			},
			labels,
		),

		WriteTimeout: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "write_timeouts_total",
				Help:      "Total number of write timeouts",
			},
			labels,
		),

		IdleTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "connection_idle_seconds",
				Help:      "Connection idle time in seconds",
				Buckets:   []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
			},
			labels,
		),

		Active: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "connection_active",
				Help:      "Connection active status (1=active, 0=idle)",
			},
			labels,
		),
	}

	// 注册所有指标
	prometheus.MustRegister(
		m.MsgTotal,
		m.MsgInTotal,
		m.MsgOutTotal,
		m.MsgDropTotal,
		m.BytesInTotal,
		m.BytesOutTotal,
		m.ErrorTotal,
		m.SendErrors,
		m.RecvErrors,
		m.CodecErrors,
		m.SendQueueSize,
		m.RecvQueueSize,
		m.LatencyHist,
		m.Connected,
		m.ConnectedTime,
		m.HeartbeatTotal,
		m.HeartbeatFailTotal,
		m.LastHeartbeat,
		m.TimeoutTotal,
		m.ReadTimeout,
		m.WriteTimeout,
		m.IdleTime,
		m.Active,
	)

	return m
}
