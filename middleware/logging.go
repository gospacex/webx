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
func Logging(l logx.Logger, stats *AccessStats) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)
		dur := time.Since(start)
		if stats != nil {
			stats.Record(ctx, c.Response.StatusCode(), dur)
		}
		l.InfoContext(ctx, "access",
			logx.String("method", string(c.Method())),
			logx.String("path", string(c.FullPath())),
			logx.Int("status", c.Response.StatusCode()),
			logx.Int("duration_ms", int(dur.Milliseconds())),
			logx.String("request_id", envelope.RequestIDFrom(c)),
			logx.String("remote", c.RemoteAddr().String()),
		)	}
}
