package middleware

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// BindMeter 经 otel/metric/noop Provider 接线 instrument（离线可用，无 sdk/metric）
func TestAccessStatsBindMeterNoop(t *testing.T) {
	s := NewAccessStats()
	if s.MetricsBound() {
		t.Fatal("fresh AccessStats: MetricsBound = true, want false")
	}
	if err := s.BindMeter(noop.NewMeterProvider().Meter("test")); err != nil {
		t.Fatalf("BindMeter: %v", err)
	}
	if !s.MetricsBound() {
		t.Fatal("after BindMeter: MetricsBound = false, want true")
	}
	// 接线后 Record 走 instrument 分支（noop 不报错）
	s.Record(context.Background(), 200, 5*time.Millisecond)
	if snap := s.Snapshot(); snap.Requests != 1 || snap.LastStatus != 200 {
		t.Fatalf("Snapshot = %+v, want Requests=1 LastStatus=200", snap)
	}
	// 重复 BindMeter 幂等（不重建 instrument）
	if err := s.BindMeter(noop.NewMeterProvider().Meter("test2")); err != nil {
		t.Fatalf("re-BindMeter: %v", err)
	}
}

// 未接线 Record 判空跳过（nil instrument 分支）
func TestAccessStatsRecordUnbound(t *testing.T) {
	s := NewAccessStats()
	s.Record(context.Background(), 500, time.Millisecond)
	snap := s.Snapshot()
	if snap.Requests != 1 || snap.LastStatus != 500 {
		t.Fatalf("Snapshot = %+v, want Requests=1 LastStatus=500", snap)
	}
}

// failingMeter 在第一个 Int64Counter 成功后让后续 instrument 创建失败，
// 用于验证 BindMeter 的 all-or-none 语义
type failingMeter struct {
	metric.Meter // 底层走 noop，保证未覆盖的方法可用
	created      int
}

func (m *failingMeter) Int64Counter(name string, opts ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	m.created++
	if m.created > 1 { // webx.requests 成功，webx.responses_by_code 失败
		return nil, fmt.Errorf("simulated instrument creation failure")
	}
	return m.Meter.Int64Counter(name, opts...)
}

func TestAccessStatsBindMeterPartialFailure(t *testing.T) {
	s := NewAccessStats()
	err := s.BindMeter(&failingMeter{Meter: noop.NewMeterProvider().Meter("fail")})
	if err == nil {
		t.Fatal("BindMeter: want error, got nil")
	}
	if s.MetricsBound() {
		t.Fatal("after partial failure: MetricsBound = true, want false（不半接线）")
	}
	// 修复后重试可正常接线（局部变量不泄漏到已提交状态）
	if err := s.BindMeter(noop.NewMeterProvider().Meter("ok")); err != nil {
		t.Fatalf("re-BindMeter after failure: %v", err)
	}
	if !s.MetricsBound() {
		t.Fatal("after re-BindMeter: MetricsBound = false, want true")
	}
}

// 已接线后重绑定不重建 instrument（noop 下断言幂等：MetricsBound 保持 true）
func TestAccessStatsBindMeterIdempotent(t *testing.T) {
	s := NewAccessStats()
	if err := s.BindMeter(noop.NewMeterProvider().Meter("m1")); err != nil {
		t.Fatalf("BindMeter: %v", err)
	}
	if err := s.BindMeter(noop.NewMeterProvider().Meter("m2")); err != nil {
		t.Fatalf("re-BindMeter: %v", err)
	}
	if !s.MetricsBound() {
		t.Fatal("after re-bind: MetricsBound = false, want true")
	}
}

// TestDurationBucketBoundsAreSecondsScale 防回归：耗时分桶必须是**秒**量级。
//
// 症状很隐蔽：分桶与单位错配不影响任何业务、不报错、单测照过，只表现为
// 「/metrics 里 p99 算不出来、所有请求挤在一个桶」。而 OTel 的缺省分桶
// [0,5,10,25,...,10000] 长得像毫秒档，最容易被顺手删掉显式分桶。
func TestDurationBucketBoundsAreSecondsScale(t *testing.T) {
	if len(durationBucketBounds) < 5 {
		t.Fatalf("分桶太少（%d 个），秒级耗时会全部落进 1-2 档", len(durationBucketBounds))
	}
	for i, b := range durationBucketBounds {
		if b <= 0 {
			t.Fatalf("分桶 %d = %v，必须为正", i, b)
		}
		if i > 0 && b <= durationBucketBounds[i-1] {
			t.Fatalf("分桶必须严格递增：%v 出现在 %v 之后", b, durationBucketBounds[i-1])
		}
	}
	if last := durationBucketBounds[len(durationBucketBounds)-1]; last > 60 {
		t.Fatalf("最大分桶 = %v 秒，量级不对（看着像毫秒档被直接搬过来了）", last)
	}
	if durationBucketBounds[0] > 0.05 {
		t.Fatalf("最小分桶 = %v 秒，粒度太粗", durationBucketBounds[0])
	}
}
