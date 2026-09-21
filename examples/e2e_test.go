// e2e 端到端调用链贯穿：TestApp 随机端口 + 真 HTTP 请求（hertz 客户端，不用 net/http）
package examples

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/gospacex/logx"
	logxzap "github.com/gospacex/logx/zap"
	"github.com/gospacex/webx"
	exapp "github.com/gospacex/webx/examples/app"
	"github.com/gospacex/webx/examples/app/handler"
	"github.com/gospacex/webx/examples/internal/logic"
	"github.com/gospacex/webx/examples/internal/middleware"
	"github.com/gospacex/webx/examples/internal/model"
	"github.com/gospacex/webx/examples/internal/service"
	"go.opentelemetry.io/otel"
)

// logEntry logx json 行日志（取断言所需字段）
type logEntry struct {
	Msg     string `json:"msg"`
	TraceID string `json:"trace_id"`
}

// readJSONLogs 读 json 行日志文件（zap 文件 sink 直写无缓冲，请求完成后即可读）
func readJSONLogs(t *testing.T, path string) []logEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log %s: %v", path, err)
	}
	defer f.Close()
	var out []logEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e logEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // 非 JSON 行（stdout exporter 输出混入等）跳过
		}
		out = append(out, e)
	}
	return out
}

// TestE2ETracePassThrough 调用链贯穿：响应 X-Request-Id == body request_id == 三层日志 trace_id
func TestE2ETracePassThrough(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "e2e.json.log")
	l, err := logx.New(logx.Config{
		Level: "info", Format: "json", Output: "file", FilePath: logPath,
		OTel: logx.OTelConfig{Enabled: true, Exporter: "stdout", ServiceName: "webx-e2e"},
	})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	// TracerProvider（stdout exporter，无外部依赖）；webx otel 中间件取全局句柄
	if _, err := logxzap.SetupTracerProvider(logx.OTelConfig{
		Enabled: true, Exporter: "stdout", ServiceName: "webx-e2e",
	}); err != nil {
		t.Fatalf("SetupTracerProvider: %v", err)
	}
	tr := otel.GetTracerProvider().Tracer("webx-e2e")

	// demo 分层装配（与 cmd/main.go 同构，测试内联）
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

	// 真 HTTP 请求
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
		Code      int    `json:"code"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(resp.Body(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("body code = %d, want 0", body.Code)
	}

	// 断言链 ① 响应头 == 响应 body
	rid := string(resp.Header.Peek("X-Request-Id"))
	if rid == "" {
		t.Fatal("response X-Request-Id header empty")
	}
	if rid != body.RequestID {
		t.Fatalf("header X-Request-Id %q != body request_id %q", rid, body.RequestID)
	}

	// 断言链 ② 三层日志 trace_id 全部 == rid（logx OTel.Enabled=true 自动附加 trace_id）
	entries := readJSONLogs(t, logPath)
	wantLayers := map[string]bool{"handler.GetOrder": false, "logic.GetOrder": false, "service.GetOrder": false}
	for _, e := range entries {
		for layer := range wantLayers {
			if strings.Contains(e.Msg, layer) {
				wantLayers[layer] = true
			}
		}
		if e.TraceID != "" && e.TraceID != rid {
			t.Fatalf("log trace_id %q != request X-Request-Id %q (msg=%s)", e.TraceID, rid, e.Msg)
		}
	}
	for layer, seen := range wantLayers {
		if !seen {
			t.Fatalf("layer %q log entry not found; got %d entries", layer, len(entries))
		}
	}
}
