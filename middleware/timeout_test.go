package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
)

// v0.10.6 实测：chain 跑完且无人写体时 hertz 默认响应为 200 Hello Hertz（非 0），
// 因此超时判定以「Next 前 deadline 是否已过期」为准（deadline 过期才 504，否则透传）。

func TestTimeoutPassesDeadlineDown(t *testing.T) {
	mw := Timeout(3*time.Second, logx.GetGlobal())
	sawDeadline := false
	handler := func(ctx context.Context, c *app.RequestContext) {
		_, ok := ctx.Deadline()
		sawDeadline = ok
		c.Next(ctx)
	}
	// 业务 handler 立即返回（deadline 未过期）：不触发 504，正常 200 透传
	got := runChain(t, mw, handler)
	if !sawDeadline {
		t.Fatal("downstream ctx has no deadline; Timeout did not wrap context")
	}
	if got.Response.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200 (no timeout on fast path)", got.Response.StatusCode())
	}
}

func TestTimeoutExceededRenders504(t *testing.T) {
	// Timeout(0)：WithTimeout(0) 产生已过期 ctx → 入链前 deadline 已过期 → 504 短路（不触达业务 handler）。
	// 注：chain 跑完且无人写体时 hertz 默认 200 填充在 Next 返回之后，post-Next 判断不可靠，
	// 故超时守卫以「入链前 deadline 是否已过期」为准（业务 handler 经 ctx deadline 自行兜底）。
	mwZero := Timeout(0, logx.GetGlobal())
	touched := false
	handler := func(ctx context.Context, c *app.RequestContext) {
		touched = true
	}
	got := runChain(t, mwZero, handler)
	if got.Response.StatusCode() != 504 {
		t.Fatalf("status = %d, want 504", got.Response.StatusCode())
	}
	if touched {
		t.Fatal("handler was touched despite expired deadline; want short-circuit 504")
	}
}
