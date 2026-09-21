package middleware

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// AccessStats webx 内置线程安全访问统计。
// 离线环境不引入 otel/sdk/metric：instrument 经 otel/metric Meter 创建（Task 5 接线），
// 测试断言走本记录器的原子快照，不依赖 otel reader。
type AccessStats struct {
	mu sync.Mutex
	// instrument 未接线前为 nil，Record 判空跳过
	reqCounter  metric.Int64Counter
	durHist     metric.Float64Histogram
	codeCounter metric.Int64Counter

	requests   int64
	lastStatus int
}

// NewAccessStats 创建未接线的 AccessStats
func NewAccessStats() *AccessStats { return &AccessStats{} }

// Snapshot 线程安全快照（测试断言用）
type StatsSnapshot struct {
	Requests   int64
	LastStatus int
}

// Snapshot 取线程安全快照
func (s *AccessStats) Snapshot() StatsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return StatsSnapshot{Requests: s.requests, LastStatus: s.lastStatus}
}

// Record 一次访问（Logging 中间件请求完成后调用；ctx 为请求 ctx，span 属性自动带入）
func (s *AccessStats) Record(ctx context.Context, status int, dur time.Duration) {
	s.mu.Lock()
	s.requests++
	s.lastStatus = status
	req, code, durH := s.reqCounter, s.codeCounter, s.durHist
	s.mu.Unlock()

	if req != nil {
		req.Add(ctx, 1, metric.WithAttributes(
			attribute.Int("http.status_code", status),
		))
	}
	if code != nil {
		code.Add(ctx, 1, metric.WithAttributes(
			attribute.Int("http.status_code", status),
		))
	}
	if durH != nil {
		durH.Record(ctx, float64(dur.Seconds()), metric.WithAttributes(
			attribute.Int("http.status_code", status),
		))
	}
}

// MetricsBound instrument 是否已接线（测试断言用）
func (s *AccessStats) MetricsBound() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reqCounter != nil
}

// BindMeter 由 webx 根包（Task 5）在 Meter 可用时接线 instrument
func (s *AccessStats) BindMeter(meter metric.Meter) error {
	var err error
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reqCounter == nil {
		s.reqCounter, err = meter.Int64Counter("webx.requests",
			metric.WithDescription("total requests"),
			metric.WithUnit("{request}"))
	}
	if err == nil && s.codeCounter == nil {
		s.codeCounter, err = meter.Int64Counter("webx.responses_by_code",
			metric.WithDescription("responses by http status code"),
			metric.WithUnit("{response}"))
	}
	if err == nil && s.durHist == nil {
		s.durHist, err = meter.Float64Histogram("webx.request.duration",
			metric.WithDescription("request duration seconds"),
			metric.WithUnit("s"))
	}
	return err
}
