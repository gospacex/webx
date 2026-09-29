package examples

// 调用链开关两态测试（6.5）：
// 关闭态 → BuildTracer 返回 noop 委托 tracer（enabled=false），发探针请求仍 200，行为不变；
// 开启态 → 注入 InMemory exporter 的 SDK provider，经真实 HTTP 请求驱动，
// 断言 span 树（root `GET /orders/1` + handler/logic/service 三层子 span 的父子关系）。

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx"
	exapp "github.com/gospacex/webx/examples/app"
	"github.com/gospacex/webx/examples/app/handler"
	"github.com/gospacex/webx/examples/internal/config"
	"github.com/gospacex/webx/examples/internal/logic"
	"github.com/gospacex/webx/examples/internal/middleware"
	"github.com/gospacex/webx/examples/internal/model"
	"github.com/gospacex/webx/examples/internal/service"
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

	// 关闭态：发探针请求仍 200——行为不变
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

	l, err := logx.New(logx.Config{
		Level: "info", Format: "json", Output: "file",
		FilePath: filepath.Join(t.TempDir(), "on.json.log"),
	})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	// demo 分层装配（与 cmd/main.go 同构）
	repo := model.NewMemRepo()
	if err := repo.Create(context.Background(), &model.Order{ID: 1, Item: "pen", Status: "pending"}); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	svc := service.New(repo, l, tr)
	logicLayer := logic.New(svc, l, tr)
	hdl := handler.New(logicLayer, l, tr)

	a, err := webx.TestApp(
		webx.WithLogger(l),
		webx.WithTracer(tr),
		webx.WithMiddleware(middleware.Auth("demo")),
		webx.WithPreRun(func(a *webx.App) { // 路由注册须早于监听（避免 hertz 路由树并发写）
			exapp.RegisterRoutes(a.Engine, hdl)
		}),
	)
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	// 真实 HTTP 请求驱动 span 树（Auth 中间件校验 X-Token）
	hc, err := client.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	req := &protocol.Request{}
	req.SetMethod("GET")
	req.SetRequestURI(a.URL() + "/orders/1")
	req.Header.Set("X-Token", "demo")
	resp := &protocol.Response{}
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode(), resp.Body())
	}
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("body code = %d, want 0", body.Code)
	}

	// 断言 span 树：root（webx otel 中间件开 `GET /orders/1`）→ 各层子 span
	provider.ForceFlush(context.Background())
	byName := map[string]tracetest.SpanStub{}
	for _, s := range exp.GetSpans() {
		byName[s.Name] = s
	}
	// 分层为嵌套调用链：root → handler → logic → service，逐层断言父子关系
	for parentName, childName := range map[string]string{
		"GET /orders/1":    "handler.GetOrder",
		"handler.GetOrder": "logic.GetOrder",
		"logic.GetOrder":   "service.GetOrder",
	} {
		parent, ok := byName[parentName]
		if !ok {
			t.Fatalf("span %q missing: %v", parentName, spanNames(exp.GetSpans()))
		}
		child, ok := byName[childName]
		if !ok {
			t.Fatalf("span %q missing: %v", childName, spanNames(exp.GetSpans()))
		}
		if child.Parent.SpanID() != parent.SpanContext.SpanID() {
			t.Fatalf("span %q parent = %s, want %q %s",
				childName, child.Parent.SpanID(), parentName, parent.SpanContext.SpanID())
		}
	}
}

// spanNames 仅取 span 名（失败信息用，避免打印全量 stub）
func spanNames(stubs []tracetest.SpanStub) []string {
	out := make([]string, 0, len(stubs))
	for _, s := range stubs {
		out = append(out, s.Name)
	}
	return out
}
