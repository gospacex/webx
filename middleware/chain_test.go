package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"go.opentelemetry.io/otel"
)

func TestDefaultChainOrder(t *testing.T) {
	tracer := otel.GetTracerProvider().Tracer("webx-test") // 未装配时返回 noop，安全
	stats := NewAccessStats()
	chain := DefaultChain(tracer, stats, logx.GetGlobal(), 10, 20, nil, 5*time.Second)
	if len(chain) != 6 {
		t.Fatalf("DefaultChain len = %d, want 6", len(chain))
	}
	// 组合跑一轮：链内无 panic/限流触发，200 正常透传
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/ordered")
	c.SetHandlers(append(chain, func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, "ok")
	}))
	c.Next(context.Background())
	if c.Response.StatusCode() != 200 {
		t.Fatalf("composed chain status = %d, want 200", c.Response.StatusCode())
	}
	if stats.Snapshot().Requests != 1 {
		t.Fatalf("logging in composed chain recorded %d, want 1", stats.Snapshot().Requests)
	}
}
