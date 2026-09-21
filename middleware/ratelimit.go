package middleware

import (
	"context"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
	"golang.org/x/time/rate"
)

// RateLimit 按 client IP 令牌桶限流；超限 → 4290/429
// 桶以请求 IP 为键（单元测试中 ClientIP 为空串，同键命中同桶）
func RateLimit(rps, burst int) app.HandlerFunc {
	if rps <= 0 {
		rps = 10
	}
	if burst <= 0 {
		burst = rps
	}
	mu := &sync.Mutex{}
	buckets := make(map[string]*rate.Limiter)
	return func(ctx context.Context, c *app.RequestContext) {
		key := string(c.ClientIP())
		mu.Lock()
		lim, ok := buckets[key]
		if !ok {
			lim = rate.NewLimiter(rate.Limit(rps), burst)
			buckets[key] = lim
		}
		mu.Unlock()
		if !lim.Allow() {
			c.Abort()
			envelope.Render(c, nil, envelope.ErrRateLimited) // code 4290 → 429
			return
		}
		c.Next(ctx)
	}
}
