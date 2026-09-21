package webx

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestOTelSpanExport root span 经 WithSyncer 同步导出；X-Request-Id == traceID
func TestOTelSpanExport(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	a, err := TestApp(WithTracer(provider.Tracer("webx-test")), WithMetrics(true))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	hc := newHertzClient(t)
	req := newProtocolRequest(t, "GET", a.URL()+"/healthz/live")
	resp := newProtocolResponse()
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("Do: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1 root span", len(spans))
	}
	got := string(resp.Header.Peek("X-Request-Id"))
	if want := spans[0].SpanContext.TraceID().String(); got != want {
		t.Fatalf("X-Request-Id = %s, want trace-id %s", got, want)
	}
}

// TestMetricsBoundOnEnable WithMetrics(true) 接线 instrument；false 不接线
func TestMetricsBoundOnEnable(t *testing.T) {
	a, err := TestApp(WithMetrics(true))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })
	if !a.Config().Stats.MetricsBound() {
		t.Fatal("AccessStats.MetricsBound() = false, want true after WithMetrics(true)")
	}

	b, err := TestApp(WithMetrics(false))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = b.Shutdown(context.Background()) })
	if b.Config().Stats.MetricsBound() {
		t.Fatal("AccessStats.MetricsBound() = true, want false (metrics disabled)")
	}
}
