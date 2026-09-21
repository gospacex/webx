package middleware

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
)

// Timeout 给下游 ctx 打 deadline；deadline 已过期（入链前或 handler 返回时）→ 5002/504。
// d <= 0 视为「零预算」→ 立即 504（DefaultChain 已把缺省归一为 5s，该分支主要供测试确定性触发）。
func Timeout(d time.Duration, l logx.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if d <= 0 {
			l.WarnContext(ctx, "request timed out before dispatch (zero budget)",
				logx.String("path", string(c.FullPath())))
			c.Abort() // 停链：hertz engine 按 IsAborted 提前终止后续 handler
			envelope.Render(c, nil, envelope.ErrTimeout) // code 5002 → 504
			return
		}
		reqCtx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		// 前置短路：入链时 deadline 已过期 → 不触达业务 handler
		if dl, ok := reqCtx.Deadline(); ok && !time.Now().Before(dl) {
			l.WarnContext(reqCtx, "request timed out before dispatch",
				logx.String("path", string(c.FullPath())))
			c.Abort()
			envelope.Render(c, nil, envelope.ErrTimeout) // code 5002 → 504
			return
		}
		c.Next(reqCtx)
		if c.IsAborted() || c.Response.StatusCode() != 0 {
			return // 下游已自行结束（含业务写体 / 其它中间件 Abort）
		}
		if dl, ok := reqCtx.Deadline(); ok && time.Now().After(dl) {
			l.WarnContext(reqCtx, "request timed out",
				logx.String("path", string(c.FullPath())))
			envelope.Render(c, nil, envelope.ErrTimeout) // code 5002 → 504
		}
	}
}
