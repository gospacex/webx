package middleware

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestRateLimitBucketEviction(t *testing.T) {
	// rps=1：无淘汰时 120ms 仅回填 0.12 token，第二请求必被拒；
	// TTL 过期后桶被清理重建（满桶），第二请求应放行
	mw := RateLimit(1, 1,
		WithRateLimitTTL(50*time.Millisecond),
		WithRateLimitSweepPeriod(time.Millisecond),
	)
	okHandler := func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }

	if got := runChain(t, mw, okHandler); got.Response.StatusCode() != 200 {
		t.Fatalf("first request status = %d, want 200", got.Response.StatusCode())
	}
	time.Sleep(120 * time.Millisecond) // 超过 TTL，且触发请求路径上的清理
	if got := runChain(t, mw, okHandler); got.Response.StatusCode() != 200 {
		t.Fatalf("request after TTL status = %d, want 200 (bucket evicted and rebuilt)", got.Response.StatusCode())
	}
}

func TestRateLimitDefaults(t *testing.T) {
	mw := RateLimit(0, 0) // rps<=0 → 10，burst<=0 → rps=10
	okHandler := func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }

	allowed := 0
	var last int
	for i := 0; i < 11; i++ {
		got := runChain(t, mw, okHandler)
		last = got.Response.StatusCode()
		if last == 200 {
			allowed++
		}
	}
	// 紧循环内回填可忽略：burst=10 应放行前 10 个、拒第 11 个
	if allowed != 10 || last != 429 {
		t.Fatalf("allowed = %d, last status = %d, want allowed=10 and last=429 (default rps=10 burst=10)", allowed, last)
	}
}

func TestRateLimitConcurrent(t *testing.T) {
	mw := RateLimit(1000000, 1000000) // 大额度，聚焦并发安全而非限流判定
	okHandler := func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }

	var denied atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				c := app.NewContext(0)
				c.Request.SetMethod("GET")
				c.Request.SetRequestURI("/ping")
				c.SetHandlers(app.HandlersChain{mw, okHandler})
				c.Next(context.Background())
				if c.Response.StatusCode() == 429 {
					denied.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if n := denied.Load(); n != 0 {
		t.Fatalf("unexpected 429 count = %d, want 0", n)
	}
}
