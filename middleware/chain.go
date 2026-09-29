package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"go.opentelemetry.io/otel/trace"
)

// DefaultChain 默认中间件顺序：recovery → otel → logging → ratelimit → cors → timeout。
// 业务 handler 与 WithMiddleware 由 webx.New 追加于链尾（3.4）。
// exempt 为限流豁免前缀（如 /healthz）：命中前缀的请求不走令牌桶，
// 保证健康探针在限流压力下仍可被探活。
func DefaultChain(tr trace.Tracer, stats *AccessStats, l logx.Logger,
	rps, burst int, corsOrigins []string, timeout time.Duration,
	exempt ...string) app.HandlersChain {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return app.HandlersChain{
		Recovery(l),
		Otel(tr),
		Logging(l, stats),
		exemptRateLimit(RateLimit(rps, burst), exempt),
		CORS(corsOrigins),
		Timeout(timeout, l),
	}
}

// exemptRateLimit 包装限流 handler：exempt 非空且路径命中前缀时直接放行
func exemptRateLimit(inner app.HandlerFunc, exempt []string) app.HandlerFunc {
	if len(exempt) == 0 {
		return inner
	}
	return func(ctx context.Context, c *app.RequestContext) {
		path := c.FullPath()
		if path == "" {
			path = string(c.Path())
		}
		for _, p := range exempt {
			if path == p || strings.HasPrefix(path, p+"/") {
				c.Next(ctx)
				return
			}
		}
		inner(ctx, c)
	}
}
