package middleware

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
	otrace "go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// otelTracerForTest 用 InMemoryExporter + Syncer 造真实 Tracer（trace-id 非全 0）
func otelTracerForTest(t *testing.T, exp *tracetest.InMemoryExporter) otrace.Tracer {
	t.Helper()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider.Tracer("webx-test")
}

// runOtelChain 跑一条「otel → passthrough handler」的 chain；inbound 非空时设置入站 X-Request-Id
func runOtelChain(t *testing.T, tr otrace.Tracer, inbound string) *app.RequestContext {
	t.Helper()
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/demo/42")
	if inbound != "" {
		c.Request.Header.Set("X-Request-Id", inbound)
	}
	c.SetHandlers(app.HandlersChain{Otel(tr), func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())
	return c
}

func TestOtelSetsRequestIDHeader(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tr := otelTracerForTest(t, exp)
	c := runOtelChain(t, tr, "")

	// 无入站 X-Request-Id → 服务端生成 32-hex trace-id
	rid := string(c.Response.Header.Peek("X-Request-Id"))
	if len(rid) != 32 {
		t.Fatalf("X-Request-Id = %q, want 32-hex trace-id", rid)
	}
	// trace-id 同步写入 hertz ctx（envelope 取用）
	if envelope.RequestIDFrom(c) != rid {
		t.Fatalf("envelope.RequestIDFrom = %q, want %q", envelope.RequestIDFrom(c), rid)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans = %+v, want 1 exported root span", spans)
	}
	if !strings.Contains(spans[0].Name, "GET") {
		t.Fatalf("span name = %q, want contains 'GET'", spans[0].Name)
	}
}

func TestOtelSeedsInboundRequestID(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tr := otelTracerForTest(t, exp)

	// 入站合法 32-hex trace-id → 响应 trace-id 与之贯穿
	const inbound = "4bf92f3577b34da6a3ce929d4e71be44"
	c1 := runOtelChain(t, tr, inbound)
	if got := string(c1.Response.Header.Peek("X-Request-Id")); got != inbound {
		t.Fatalf("X-Request-Id = %q, want inbound %q (trace-id 贯穿)", got, inbound)
	}
	if envelope.RequestIDFrom(c1) != inbound {
		t.Fatalf("envelope.RequestIDFrom = %q, want %q", envelope.RequestIDFrom(c1), inbound)
	}
	// span 亦以入站 trace-id 为根
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	if spans[0].SpanContext.TraceID().String() != inbound {
		t.Fatalf("span trace-id = %q, want inbound %q", spans[0].SpanContext.TraceID().String(), inbound)
	}

	// 入站非法值（非 32-hex）→ 服务端生成，不静默吞
	c2 := runOtelChain(t, tr, "not-a-trace-id")
	if got := string(c2.Response.Header.Peek("X-Request-Id")); got == "not-a-trace-id" || len(got) != 32 {
		t.Fatalf("invalid inbound: X-Request-Id = %q, want server-generated 32-hex", got)
	}
}
