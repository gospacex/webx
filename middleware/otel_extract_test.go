package middleware

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestOtelExtractsTraceparent：入站带合法 traceparent 时，Otel 产出的 span
// 必须是该上游 trace 的子级（TraceID 一致）——gateway→BFF 续链的落点。
func TestOtelExtractsTraceparent(t *testing.T) {
	// OTel 全局传播器缺省是 noop（既不注入也不提取），而 Extract 走全局传播器，
	// 故测试需按消费方（micro_apps infra/telemetry）的方式显式安装 W3C 传播器。
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	// 上游 trace：00-<tid>-<sid>-01
	const tid = "0af7651916cd43dd8448eb211c80319c"
	const sid = "b7ad6b7169203331"
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/demo/42")
	c.Request.Header.Set("traceparent", "00-"+tid+"-"+sid+"-01")

	Otel(tp.Tracer("test"))(context.Background(), c)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if got := spans[0].SpanContext.TraceID().String(); got != tid {
		t.Fatalf("span trace id %q not continued from inbound traceparent %q", got, tid)
	}
	if spans[0].Parent.SpanID().String() != sid {
		t.Fatalf("parent span id not extracted from traceparent")
	}
}
