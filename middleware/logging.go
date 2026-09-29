package middleware

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
)

// Logging access 日志：method/path/status/duration_ms/request_id/remote，
// 经 logx InfoContext 输出（caller 归因到本中间件），并记入 AccessStats（指标钩子）。
// 记录逻辑在 defer 中执行：下游 panic 时（Recovery 在外层兜底渲染之前）也不漏
// access 日志与统计，记录完成后原样 re-panic 交外层处理。
func Logging(l logx.Logger, stats *AccessStats) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		defer func() {
			dur := time.Since(start)
			status := c.Response.StatusCode()
			r := recover()
			// panic 展开期间响应头尚未写 5xx——记为 500（与外层 Recovery 渲染的 5001/500 一致）
			if r != nil && status < 500 {
				status = 500
			}
			path := c.FullPath()
			if len(path) == 0 { // 404 未命中路由时 FullPath 为空，回退原始 path
				path = string(c.Path())
			}
			if stats != nil {
				stats.Record(ctx, status, dur)
			}
			l.InfoContext(ctx, "access",
				logx.String("method", string(c.Method())),
				logx.String("path", string(path)),
				logx.Int("status", status),
				logx.Int("duration_ms", int(dur.Milliseconds())),
				logx.String("request_id", envelope.RequestIDFrom(c)),
				logx.String("remote", c.RemoteAddr().String()),
			)
			if r != nil {
				panic(r)
			}
		}()
		c.Next(ctx)
	}
}
