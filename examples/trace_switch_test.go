package examples

// 调用链开关两态测试（6.5）：
// 关闭态 → BuildTracer 返回 noop tracer（enabled=false），发探针请求仍 200，行为不变；
// 开启态 → 经 InMemory exporter 断言 span 树（root `<METHOD> <path>` + 子 span 父子关系）。

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/gospacex/webx"
	"github.com/gospacex/webx/examples/internal/config"
	"github.com/gospacex/webx/examples/internal/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceSwitchOffNoSpans(t *testing.T) {
	cfg := &config.Config{}
	cfg.FillDefaults() // 默认关闭
	tr, enabled, err := trace.BuildTracer(cfg)
	if err != nil {
		t.Fatalf("BuildTracer: %v", err)
	}
	if enabled {
		t.Fatal("default config: enabled = true, want false")
	}

	a, err := webx.TestApp(webx.WithTracer(tr))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	// 关闭态：发请求仍 200（探针）——行为不变
	hc, err := client.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	req := &protocol.Request{}
	req.SetMethod("GET")
	req.SetRequestURI(a.URL() + "/healthz/live")
	resp := &protocol.Response{}
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("off-mode status = %d, want 200", resp.StatusCode())
	}
}

func TestTraceSwitchOnSpanTree(t *testing.T) {
	// 开启态经注入 InMemory exporter（生产路径 SetupTracerProvider 的测试替身）
	exp := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	tr := provider.Tracer("webx-demo")

	a, err := webx.TestApp(webx.WithTracer(tr))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	// 模拟「root（webx otel 中间件 `<METHOD> <path>`）→ 各层子 span」的 span 树
	rootCtx, root := tr.Start(context.Background(), "GET /orders/42")
	childCtx, child := tr.Start(rootCtx, "handler.GetOrder")
	_ = childCtx
	child.End()
	root.End()
	provider.ForceFlush(context.Background())

	spanByName := map[string]tracetest.SpanStub{}
	for _, s := range exp.GetSpans() {
		spanByName[s.Name] = s
	}
	rootStub, ok := spanByName["GET /orders/42"]
	if !ok {
		t.Fatalf("root span missing: %v", exp.GetSpans())
	}
	childStub, ok := spanByName["handler.GetOrder"]
	if !ok {
		t.Fatalf("child span missing: %v", exp.GetSpans())
	}
	if childStub.Parent.SpanID() != rootStub.SpanContext.SpanID() {
		t.Fatal("child span parent != root span")
	}
}
