package middleware

import (
	"context"
	"strings"
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
