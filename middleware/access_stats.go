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

// durationBucketBounds 耗时分桶（单位：秒）。
//
// **必须显式给**：OTel SDK 的缺省分桶是 [0, 5, 10, 25, ..., 10000]，那是**毫秒**
// 形状的数值，而本直方图声明的单位是 `s`。错配的后果不是报错而是**分桶全废** ——
// 请求耗时几乎全落在 `le="5"` 这一档，等于只有一个桶，快慢与 p99 都读不出来。
// 取值按 HTTP 请求的现实量级铺：亚毫秒到 5 秒，10 秒封顶。
var durationBucketBounds = []float64{
	0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

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

// BindMeter 由 webx 根包（Task 5）在 Meter 可用时接线 instrument。
// all-or-none：先建局部 instrument，全部成功后一次性提交——
// 部分失败不留下半接线状态（否则 reqCounter 已接而代码统计缺失，
// 却仍被 MetricsBound 报 true）。已接线时重绑定幂等：不重建、不报错。
func (s *AccessStats) BindMeter(meter metric.Meter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.MetricsBoundLocked() {
		return nil
	}
	reqCounter, err := meter.Int64Counter("webx.requests",
		metric.WithDescription("total requests"),
		metric.WithUnit("{request}"))
	var codeCounter metric.Int64Counter
	if err == nil {
		codeCounter, err = meter.Int64Counter("webx.responses_by_code",
			metric.WithDescription("responses by http status code"),
			metric.WithUnit("{response}"))
	}
	var durHist metric.Float64Histogram
	if err == nil {
		durHist, err = meter.Float64Histogram("webx.request.duration",
			metric.WithDescription("request duration seconds"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(durationBucketBounds...))
	}
	if err != nil {
		return err
	}
	s.reqCounter, s.codeCounter, s.durHist = reqCounter, codeCounter, durHist
	return nil
}

// MetricsBoundLocked instrument 是否全部接线（调用方须持有 mu）
func (s *AccessStats) MetricsBoundLocked() bool {
	return s.reqCounter != nil && s.codeCounter != nil && s.durHist != nil
}

// MetricsBound instrument 是否全部接线（测试断言用）
func (s *AccessStats) MetricsBound() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.MetricsBoundLocked()
}
