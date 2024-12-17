package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type RateLimiterMetrics struct {
	labelNames  []string
	labelValues []string
	requests    *prometheus.CounterVec
	waitTime    *prometheus.HistogramVec
	state       *prometheus.GaugeVec
	tokens      *prometheus.GaugeVec
}

const (
	ResultAllowed  = "allowed"
	ResultRejected = "rejected"
	ResultTimeout  = "timeout"
)

func NewRateLimiterMetrics(namespace, subsystem string, constLabels prometheus.Labels) *RateLimiterMetrics {
	// 从 constLabels 提取标签名和值
	labelNames := make([]string, 0, len(constLabels))
	labelValues := make([]string, 0, len(constLabels))

	for name, value := range constLabels {
		labelNames = append(labelNames, name)
		labelValues = append(labelValues, value)
	}

	m := &RateLimiterMetrics{
		labelNames:  labelNames,
		labelValues: labelValues,
	}

	m.requests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "requests_total",
			Help:        "Total number of requests by result",
			ConstLabels: constLabels,
		},
		[]string{"result"},
	)

	m.waitTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "wait_seconds",
			Help:        "Wait time in seconds by result",
			Buckets:     []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
			ConstLabels: constLabels,
		},
		[]string{"result"},
	)

	m.state = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "state",
			Help:        "Current state of rate limiter",
			ConstLabels: constLabels,
		},
		[]string{"type"},
	)

	m.tokens = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "tokens",
			Help:        "Token bucket metrics",
			ConstLabels: constLabels,
		},
		[]string{"type"},
	)

	return m
}

// IncRequests 增加请求计数
func (m *RateLimiterMetrics) IncRequests(result string) {
	m.requests.WithLabelValues(result).Inc()
}

// ObserveWaitTime 记录等待时间
func (m *RateLimiterMetrics) ObserveWaitTime(seconds float64, result string) {
	m.waitTime.WithLabelValues(result).Observe(seconds)
}

// SetRate 设置速率
func (m *RateLimiterMetrics) SetRate(value float64) {
	m.state.WithLabelValues("rate").Set(value)
}

// SetBurst 设置突发值
func (m *RateLimiterMetrics) SetBurst(value float64) {
	m.state.WithLabelValues("burst").Set(value)
}

// SetWaitingCount 设置等待计数
func (m *RateLimiterMetrics) SetWaitingCount(value float64) {
	m.state.WithLabelValues("waiting").Set(value)
}

// SetTokensAvailable 设置可用令牌数
func (m *RateLimiterMetrics) SetTokensAvailable(value float64) {
	m.tokens.WithLabelValues("available").Set(value)
}

// SetTokensTotal 设置总令牌数
func (m *RateLimiterMetrics) SetTokensTotal(value float64) {
	m.tokens.WithLabelValues("total").Set(value)
}
