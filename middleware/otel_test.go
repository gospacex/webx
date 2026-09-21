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

func TestOtelSetsRequestIDHeader(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tr := otelTracerForTest(t, exp)
	mw := Otel(tr)
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/demo/42")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())

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
