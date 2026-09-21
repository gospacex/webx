package middleware

import (
	"context"
	"testing"
	"time"

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
