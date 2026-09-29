package middleware

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
)

// runChain 链单测公共驱动：mw 后接 handler，跑一轮 hertz handler chain。
func runChain(t *testing.T, mw app.HandlerFunc, handler app.HandlerFunc) *app.RequestContext {
	t.Helper()
	ctx := app.NewContext(0)
	ctx.Request.SetMethod("GET")
	ctx.Request.SetRequestURI("/boom")
	ctx.SetHandlers(app.HandlersChain{mw, handler})
	ctx.Next(context.Background())
	return ctx
}

// fakeLogger 捕获 Error / ErrorContext 调用的 logx.Logger
// （embedding 全局 logger 兜底其余接口方法，仅覆写两个 Error* 入口）
type fakeLogger struct {
	logx.Logger

	mu      sync.Mutex
	errs    []fakeErrCall
	errCtxt []fakeErrCall
}

type fakeErrCall struct {
	msg    string
	fields []logx.Field
}

func newFakeLogger() *fakeLogger {
	return &fakeLogger{Logger: logx.GetGlobal()}
}

func (f *fakeLogger) Error(msg string, fields ...logx.Field) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, fakeErrCall{msg, fields})
}

func (f *fakeLogger) ErrorContext(_ context.Context, msg string, fields ...logx.Field) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errCtxt = append(f.errCtxt, fakeErrCall{msg, fields})
}

// fieldByKey 从日志字段里取指定 key 的 zap.Field（不存在返回 false）
func fieldByKey(fields []logx.Field, key string) (logx.Field, bool) {
	for _, fd := range fields {
		if fd.Key == key {
			return fd, true
		}
	}
	return logx.Field{}, false
}

func TestRecoveryRecoversPanic(t *testing.T) {
	mw := Recovery(logx.GetGlobal())
	handler := func(ctx context.Context, c *app.RequestContext) { panic("demo panic") }
	got := runChain(t, mw, handler)

	if got.Response.StatusCode() != 500 {
		t.Fatalf("status = %d, want 500", got.Response.StatusCode())
	}
	body := string(got.Response.Body())
	if !strings.Contains(body, `"code":5001`) {
		t.Fatalf("body = %s, want envelope code 5001", body)
	}
}

// TestRecoveryLogsPanicValueAndStack 验证 panic 值与 stack 帧写入 error 日志字段
func TestRecoveryLogsPanicValueAndStack(t *testing.T) {
	fake := newFakeLogger()
	mw := Recovery(fake)
	handler := func(ctx context.Context, c *app.RequestContext) { panic("kaboom-123") }
	runChain(t, mw, handler)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.errCtxt) != 1 {
		t.Fatalf("ErrorContext calls = %d, want 1 (panic 兜底应恰好记一条 error)", len(fake.errCtxt))
	}
	call := fake.errCtxt[0]
	if call.msg != "recovered panic" {
		t.Fatalf("log msg = %q, want %q", call.msg, "recovered panic")
	}

	// panic 值原样入字段（zap.Any(string) 走 String 分支）
	pf, ok := fieldByKey(call.fields, "panic")
	if !ok {
		t.Fatalf("fields = %+v, want 含 key=panic", call.fields)
	}
	if pf.String != "kaboom-123" {
		t.Fatalf("panic field = %q, want %q", pf.String, "kaboom-123")
	}

	// stack 字段是 debug.Stack 输出，必须含 goroutine 帧文本
	sf, ok := fieldByKey(call.fields, "stack")
	if !ok {
		t.Fatalf("fields = %+v, want 含 key=stack", call.fields)
	}
	if !strings.Contains(sf.String, "goroutine") {
		t.Fatalf("stack field = %q, want 含 goroutine 帧", sf.String)
	}
}
