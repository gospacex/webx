package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/gospacex/logx"
	"go.opentelemetry.io/otel"
)

// 契约：nil 依赖 = fail fast 1001（handler 前置短路，不 panic）
func TestHandlerRendersEnvelope(t *testing.T) {
	h := New(nil, logx.GetGlobal(), otel.GetTracerProvider().Tracer("t"))

	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/orders/42")
	c.Params = param.Params{{Key: "id", Value: "42"}} // 直接赋值注入 param（v0.10.6 验证）
	c.SetHandlers(app.HandlersChain{h.GetOrder})
	c.Next(context.Background())

	body := string(c.Response.Body())
	if !strings.Contains(body, `"code":1001`) {
		t.Fatalf("body = %s, want code 1001 (nil logic fail fast)", body)
	}
	if c.Response.StatusCode() != 400 {
		t.Fatalf("status = %d, want 400", c.Response.StatusCode())
	}
}
