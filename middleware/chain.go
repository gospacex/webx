package middleware

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"go.opentelemetry.io/otel/trace"
)

// DefaultChain 默认中间件顺序：recovery → otel → logging → ratelimit → cors → timeout。
// 业务 handler 与 WithMiddleware 由 webx.New 追加于链尾（3.4）。
func DefaultChain(tr trace.Tracer, stats *AccessStats, l logx.Logger,
	rps, burst int, corsOrigins []string, timeout time.Duration) app.HandlersChain {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return app.HandlersChain{
		Recovery(l),
		Otel(tr),
		Logging(l, stats),
		RateLimit(rps, burst),
		CORS(corsOrigins),
		Timeout(timeout, l),
	}
}
