package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var (
	tcpServerMetricsInstance *TcpServerMetrics
	tcpServerMetricsOnce     sync.Once
)

// GetTcpServerMetrics 获取服务器指标单例
func GetTcpServerMetrics(namespace string) *TcpServerMetrics {
	tcpServerMetricsOnce.Do(func() {
		tcpServerMetricsInstance = newServerMetrics(namespace)
	})
	return tcpServerMetricsInstance
}

// TcpServerMetrics 服务器相关的监控指标
type TcpServerMetrics struct {
	// 服务器状态指标
	ServerStatus   prometheus.Gauge   // 服务器运行状态(0:停止,1:运行)
	StartupTime    prometheus.Gauge   // 服务器启动时间戳
	UptimeSeconds  prometheus.Gauge   // 服务器运行时长(秒)
	StartupErrors  prometheus.Counter // 启动错误计数
	ShutdownErrors prometheus.Counter // 关闭错误计数

	// 连接基础指标
	AcceptTotal    prometheus.Counter   // 接受连接总数
	AcceptErrors   prometheus.Counter   // 接受连接错误数
	AcceptDuration prometheus.Histogram // 接受连接耗时分布

	// 连接状态指标
	ConnectionGauge       prometheus.Gauge // 当前连接总数
	ActiveConnectionGauge prometheus.Gauge // 当前活跃连接数
	ConnectionUtilization prometheus.Gauge // 连接使用率

	// 连接生命周期指标
	ConnectionsCreatedTotal prometheus.Counter   // 创建的连接总数
	ConnectionsClosedTotal  prometheus.Counter   // 关闭的连接总数
	ConnectionCloseErrors   prometheus.Counter   // 连接关闭错误数
	ConnectionLifetime      prometheus.Histogram // 连接生命周期分布

	// 流量指标
	BytesInTotal  prometheus.Counter // 总接收字节数
	BytesOutTotal prometheus.Counter // 总发送字节数

	// 消息指标
	BroadcastTotal       prometheus.Counter   // 广播消息总数
	BroadcastErrors      prometheus.Counter   // 广播错误数
	BroadcastDuration    prometheus.Histogram // 广播耗时分布
	BroadcastSuccessRate prometheus.Gauge     // 广播成功率

	// 关闭指标
	ShutdownDuration prometheus.Histogram // 关闭耗时分布
}

func newServerMetrics(namespace string) *TcpServerMetrics {
	m := &TcpServerMetrics{
		// 服务器状态指标
		ServerStatus: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "server_status",
			Help:      "Server running status (0:stopped, 1:running)",
		}),
		StartupTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "startup_timestamp_seconds",
			Help:      "Server startup timestamp in seconds",
		}),
		UptimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "uptime_seconds",
			Help:      "Server uptime in seconds",
		}),
		StartupErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "startup_errors_total",
			Help:      "Total number of startup errors",
		}),
		ShutdownErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "shutdown_errors_total",
			Help:      "Total number of shutdown errors",
		}),

		// 连接基础指标
		AcceptTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "accept_total",
			Help:      "Total number of accepted connections",
		}),
		AcceptErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "accept_errors_total",
			Help:      "Total number of accept errors",
		}),
		AcceptDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "accept_duration_seconds",
			Help:      "Time spent accepting new connections",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1},
		}),

		// 连接状态指标
		ConnectionGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "connections_current",
			Help:      "Current number of connections",
		}),
		ActiveConnectionGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "connections_active",
			Help:      "Current number of active connections",
		}),
		ConnectionUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "connection_utilization",
			Help:      "Connection utilization rate",
		}),

		// 连接生命周期指标
		ConnectionsCreatedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connections_created_total",
			Help:      "Total number of connections created",
		}),
		ConnectionsClosedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connections_closed_total",
			Help:      "Total number of connections closed",
		}),
		ConnectionCloseErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connection_close_errors_total",
			Help:      "Total number of connection close errors",
		}),
		ConnectionLifetime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "connection_lifetime_seconds",
			Help:      "Connection lifetime in seconds",
			Buckets:   []float64{1, 10, 60, 300, 600, 1800, 3600},
		}),

		// 流量指标
		BytesInTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "bytes_received_total",
			Help:      "Total bytes received",
		}),
		BytesOutTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "bytes_sent_total",
			Help:      "Total bytes sent",
		}),

		// 消息指标
		BroadcastTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "broadcast_total",
			Help:      "Total number of broadcast messages",
		}),
		BroadcastErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "broadcast_errors_total",
			Help:      "Total number of broadcast errors",
		}),
		BroadcastDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "broadcast_duration_seconds",
			Help:      "Time spent broadcasting messages",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1},
		}),
		BroadcastSuccessRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "broadcast_success_rate",
			Help:      "Broadcast success rate",
		}),

		// 关闭指标
		ShutdownDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "shutdown_duration_seconds",
			Help:      "Time spent shutting down the server",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5},
		}),
	}

	// 注册所有metrics
	prometheus.MustRegister(
		m.ServerStatus,
		m.StartupTime,
		m.UptimeSeconds,
		m.StartupErrors,
		m.ShutdownErrors,
		m.AcceptTotal,
		m.AcceptErrors,
		m.AcceptDuration,
		m.ConnectionGauge,
		m.ActiveConnectionGauge,
		m.ConnectionUtilization,
		m.ConnectionsCreatedTotal,
		m.ConnectionsClosedTotal,
		m.ConnectionCloseErrors,
		m.ConnectionLifetime,
		m.BytesInTotal,
		m.BytesOutTotal,
		m.BroadcastTotal,
		m.BroadcastErrors,
		m.BroadcastDuration,
		m.BroadcastSuccessRate,
		m.ShutdownDuration,
	)

	return m
}
