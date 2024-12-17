package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var (
	metricsInstance *ConnPoolMetrics
	metricsOnce     sync.Once
)

// GetConnPoolMetrics 获取连接池指标单例
func GetConnPoolMetrics() *ConnPoolMetrics {
	metricsOnce.Do(func() {
		metricsInstance = newConnPoolMetrics("conn_pool")
	})
	return metricsInstance
}

// ConnPoolMetrics 连接池相关的监控指标
type ConnPoolMetrics struct {
	// 连接数指标
	TotalConns  prometheus.Gauge // 当前总连接数
	ActiveConns prometheus.Gauge // 当前活跃连接数
	IdleConns   prometheus.Gauge // 当前空闲连接数

	// 错误指标
	ConnErrors  prometheus.Counter // 连接错误总数
	CloseErrors prometheus.Counter // 关闭错误总数

	// 生命周期指标
	ConnCreated prometheus.Counter // 创建的连接总数
	ConnClosed  prometheus.Counter // 关闭的连接总数

	// 性能指标
	ConnAcquired      prometheus.Counter   // 获取连接总次数
	ConnAcquireFailed prometheus.Counter   // 获取连接失败总次数
	AcquireTime       prometheus.Histogram // 获取连接耗时分布

	// 清理指标
	CleanupTotal prometheus.Counter // 清理操作总次数
	CleanupConns prometheus.Counter // 被清理的连接总数
}

func newConnPoolMetrics(namespace string) *ConnPoolMetrics {
	m := &ConnPoolMetrics{
		TotalConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "connPool_connections_total",
			Help:      "Total number of connections in the pool",
		}),

		ActiveConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "connPool_connections_active",
			Help:      "Number of active connections in the pool",
		}),

		IdleConns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "connPool_connections_idle",
			Help:      "Number of idle connections in the pool",
		}),

		ConnErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connPool_connection_errors_total",
			Help:      "Total number of connection errors",
		}),

		CloseErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connPool_close_errors_total",
			Help:      "Total number of connection close errors",
		}),

		ConnCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connPool_connections_created_total",
			Help:      "Total number of connections created",
		}),

		ConnClosed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connPool_connections_closed_total",
			Help:      "Total number of connections closed",
		}),

		ConnAcquired: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connPool_connections_acquired_total",
			Help:      "Total number of connections acquired",
		}),

		ConnAcquireFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connPool_connections_acquire_failed_total",
			Help:      "Total number of failed connection acquisitions",
		}),

		AcquireTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "connPool_acquire_duration_seconds",
			Help:      "Time spent acquiring a connection",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}),

		CleanupTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connPool_cleanup_operations_total",
			Help:      "Total number of cleanup operations",
		}),

		CleanupConns: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connPool_connections_cleaned_total",
			Help:      "Total number of connections removed by cleanup",
		}),
	}

	// 注册所有metrics
	prometheus.MustRegister(m.TotalConns)
	prometheus.MustRegister(m.ActiveConns)
	prometheus.MustRegister(m.IdleConns)
	prometheus.MustRegister(m.ConnErrors)
	prometheus.MustRegister(m.CloseErrors)
	prometheus.MustRegister(m.ConnCreated)
	prometheus.MustRegister(m.ConnClosed)
	prometheus.MustRegister(m.ConnAcquired)
	prometheus.MustRegister(m.ConnAcquireFailed)
	prometheus.MustRegister(m.AcquireTime)
	prometheus.MustRegister(m.CleanupTotal)
	prometheus.MustRegister(m.CleanupConns)

	return m
}
