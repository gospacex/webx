package middleware

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestRateLimitAllowsThenDenies(t *testing.T) {
	mw := RateLimit(1, 1) // rps=1 burst=1：同 IP 首请求放行，次请求拒
	okHandler := func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }
	// 两请求同 IP（NewContext 未设连接 → ClientIP 为空串，同键命中同桶）
	got := runChain(t, mw, okHandler)
	// 放行路径无业务写体 → hertz 默认 200（v0.10.6 chain 跑完默认响应）
	if got.Response.StatusCode() != 200 {
		t.Fatalf("first request status = %d, want 200 (allowed)", got.Response.StatusCode())
	}
	got2 := runChain(t, mw, okHandler)
	if got2.Response.StatusCode() != 429 {
		t.Fatalf("second request status = %d, want 429", got2.Response.StatusCode())
	}
}
