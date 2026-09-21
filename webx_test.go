package webx

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	pclient "github.com/cloudwego/hertz/pkg/protocol/client" // 别名：与 app/client 同包名 client，避免冲突
)

// getURL 测试内 HTTP 请求（hertz 官方客户端，不用 net/http）
func getURL(t *testing.T, url string) (int, []byte, error) {
	t.Helper()
	hc, err := client.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return pclient.GetURL(context.Background(), nil, url, hc)
}

func TestNewDefaultsNoPanic(t *testing.T) {
	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cfg := a.Config()
	if cfg.Addr != "0.0.0.0:8080" {
		t.Errorf("default Addr = %q, want 0.0.0.0:8080", cfg.Addr)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("default Timeout = %s, want 5s", cfg.Timeout)
	}
	if cfg.HealthPrefix != "/healthz" {
		t.Errorf("default HealthPrefix = %q, want /healthz", cfg.HealthPrefix)
	}
	if cfg.Stats == nil || cfg.Tracer == nil {
		t.Error("default Stats/Tracer nil")
	}
}

func TestAppSingletonSamePointer(t *testing.T) {
	a1, err := AppSingleton()
	if err != nil {
		t.Fatalf("AppSingleton: %v", err)
	}
	a2, _ := AppSingleton()
	if a1 != a2 {
		t.Fatal("AppSingleton returned different pointers")
	}
}

func TestTestAppURLAndLiveProbe(t *testing.T) {
	a, err := TestApp()
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() {
		_ = a.Shutdown(context.Background())
	})
	if !strings.HasPrefix(a.URL(), "http://127.0.0.1:") {
		t.Fatalf("URL = %s, want http://127.0.0.1:<port>", a.URL())
	}
	code, body, err := getURL(t, a.URL()+"/healthz/live")
	if err != nil {
		t.Fatalf("GET live: %v", err)
	}
	if code != 200 || !strings.Contains(string(body), "alive") {
		t.Fatalf("live probe = %d %s, want 200 alive", code, body)
	}
}

func TestReadyGate(t *testing.T) {
	a, err := TestApp(WithReadyCheck(func(context.Context) error {
		return errors.New("db down")
	}))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() {
		_ = a.Shutdown(context.Background())
	})
	liveCode, _, _ := getURL(t, a.URL()+"/healthz/live")
	if liveCode != 200 {
		t.Fatalf("live = %d, want 200", liveCode)
	}
	readyCode, readyBody, _ := getURL(t, a.URL()+"/healthz/ready")
	if readyCode != 503 || !strings.Contains(string(readyBody), "unavailable") {
		t.Fatalf("ready = %d %s, want 503 unavailable", readyCode, readyBody)
	}
}

func TestGracefulShutdownDrain(t *testing.T) {
	a, err := TestApp(WithPreRun(func(a *App) { // 路由注册须早于监听（避免路由树并发读写 race）
		a.Engine.GET("/sleep", func(ctx context.Context, c *app.RequestContext) {
			time.Sleep(300 * time.Millisecond)
			c.JSON(200, "done")
		})
	}))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	done := make(chan int, 1)
	go func() {
		code, _, err := getURL(t, a.URL()+"/sleep")
		if err != nil {
			done <- 0
			return
		}
		done <- code
	}()
	// 等待请求入链（handler 300ms 睡眠）后再停机，保证排空的是真实 in-flight 请求
	time.Sleep(150 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := <-done; got != 200 {
		t.Fatalf("drained request status = %d, want 200", got)
	}
	if a.IsRunning() {
		t.Fatal("IsRunning() still true after Shutdown")
	}
}

func TestWithMiddlewareAppendedAtTail(t *testing.T) {
	a, err := TestApp(WithMiddleware(func(ctx context.Context, c *app.RequestContext) {
		c.Response.Header.Set("X-Tail", "1")
		c.Next(ctx)
	}))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() {
		_ = a.Shutdown(context.Background())
	})
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
	if got := string(resp.Header.Peek("X-Tail")); got != "1" {
		t.Fatalf("X-Tail = %q, want 1 (middleware appended at chain tail)", got)
	}
}
