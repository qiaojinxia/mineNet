package network

import (
	"context"
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	"mineNet/metrics"
	"sync"
	"time"
)

var (
	rateLimiterInstance *RateLimiter
	once                sync.Once
)

type RateLimiter struct {
	limiter   *rate.Limiter
	burstSize int
	metrics   *metrics.RateLimiterMetrics
	mu        sync.RWMutex
}

// GetRateLimiter 获取单例实例
func GetRateLimiter(rps float64, burstSize int) *RateLimiter {
	once.Do(func() {
		rateLimiterInstance = newRateLimiter(rps, burstSize)
	})
	return rateLimiterInstance
}

func newRateLimiter(rps float64, burstSize int) *RateLimiter {
	r := &RateLimiter{
		limiter:   rate.NewLimiter(rate.Limit(rps), burstSize),
		burstSize: burstSize,
	}

	// 默认指标标签
	defaultLabels := prometheus.Labels{
		"instance": "global",
	}
	r.metrics = metrics.NewRateLimiterMetrics("rate_limiter", "traffic", defaultLabels)

	// 设置初始状态
	r.metrics.SetRate(rps)
	r.metrics.SetBurst(float64(burstSize))
	r.metrics.SetTokensTotal(float64(burstSize))

	// 启动指标采集
	go r.collectMetrics()

	return r
}

func (r *RateLimiter) Allow() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allowed := r.limiter.Allow()

	if allowed {
		r.metrics.IncRequests(metrics.ResultAllowed)
	} else {
		r.metrics.IncRequests(metrics.ResultRejected)
	}

	return allowed
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.metrics.SetWaitingCount(1)
	defer r.metrics.SetWaitingCount(0)

	start := time.Now()
	err := r.limiter.Wait(ctx)

	elapsed := time.Since(start).Seconds()

	if err == nil {
		r.metrics.IncRequests(metrics.ResultAllowed)
		r.metrics.ObserveWaitTime(elapsed, metrics.ResultAllowed)
	} else if errors.Is(err, context.DeadlineExceeded) {
		r.metrics.IncRequests(metrics.ResultTimeout)
		r.metrics.ObserveWaitTime(elapsed, metrics.ResultTimeout)
	} else {
		r.metrics.IncRequests(metrics.ResultRejected)
		r.metrics.ObserveWaitTime(elapsed, metrics.ResultRejected)
	}

	return err
}

func (r *RateLimiter) collectMetrics() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.RLock()
		tokens := r.limiter.Tokens()
		r.mu.RUnlock()
		r.metrics.SetTokensAvailable(tokens)
	}
}

func (r *RateLimiter) SetRate(rps float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limiter.SetLimit(rate.Limit(rps))
	r.metrics.SetRate(rps)
}

func (r *RateLimiter) SetBurst(burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limiter.SetBurst(burst)
	r.metrics.SetBurst(float64(burst))
	r.metrics.SetTokensTotal(float64(burst))
}

func (r *RateLimiter) GetStats() (rps float64, burst int, tokens float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return float64(r.limiter.Limit()), r.limiter.Burst(), r.limiter.Tokens()
}
