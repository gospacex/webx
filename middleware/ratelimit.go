package middleware

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
	"golang.org/x/time/rate"
)

// 桶淘汰默认值：桶超过 ttl 无访问即被清理，清理动作至多每 sweep 一次
const (
	defaultBucketTTL   = 10 * time.Minute
	defaultSweepPeriod = time.Minute
)

type rateLimitConfig struct {
	ttl   time.Duration
	sweep time.Duration
}

// RateLimitOption 调整桶淘汰策略
type RateLimitOption func(*rateLimitConfig)

// WithRateLimitTTL 设置桶空闲过期时间（<=0 用默认 10min）
func WithRateLimitTTL(ttl time.Duration) RateLimitOption {
	return func(c *rateLimitConfig) { c.ttl = ttl }
}

// WithRateLimitSweepPeriod 设置过期桶的清理周期（<=0 用默认 1min）
func WithRateLimitSweepPeriod(d time.Duration) RateLimitOption {
	return func(c *rateLimitConfig) { c.sweep = d }
}

type ipBucket struct {
	lim  *rate.Limiter // 本身线程安全
	last atomic.Int64  // 最近访问时间 unixnano
}

// RateLimit 按 client IP 令牌桶限流；超限 → 4290/429
//
// 默认值：rps<=0 → 10，burst<=0 → rps。
//
// 桶表：sync.Map 无全局互斥锁（rate.Limiter 自带并发安全）；
// 每桶记录最近访问时间，空闲超过 TTL 的桶在请求路径上按周期清理，
// 防止随机源 IP / X-Forwarded-For 打爆内存。
//
// 注意：分桶键为 c.ClientIP()，hertz 默认信任任意来源的
// X-Forwarded-For（信任 CIDR 0.0.0.0/0）。部署在公网时必须置于
// 可信 LB/反代之后，或自行配置可信代理（SetClientIPFuncAndOptions /
// engine.SetClientIPFunc + TrustedProxies），否则攻击者可用随机
// XFF 头绕过限流。
func RateLimit(rps, burst int, opts ...RateLimitOption) app.HandlerFunc {
	if rps <= 0 {
		rps = 10
	}
	if burst <= 0 {
		burst = rps
	}
	cfg := rateLimitConfig{ttl: defaultBucketTTL, sweep: defaultSweepPeriod}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.ttl <= 0 {
		cfg.ttl = defaultBucketTTL
	}
	if cfg.sweep <= 0 {
		cfg.sweep = defaultSweepPeriod
	}

	var buckets sync.Map // string -> *ipBucket
	var lastSweep atomic.Int64
	lastSweep.Store(time.Now().UnixNano())

	return func(ctx context.Context, c *app.RequestContext) {
		now := time.Now().UnixNano()
		// 先清理过期桶再取桶：避免本次访问把过期桶的时间戳刷新导致漏清
		if ls := lastSweep.Load(); now-ls > int64(cfg.sweep) &&
			lastSweep.CompareAndSwap(ls, now) {
			buckets.Range(func(k, v any) bool {
				if now-v.(*ipBucket).last.Load() > int64(cfg.ttl) {
					// CompareAndDelete：仅当未被并发替换才删
					buckets.CompareAndDelete(k, v)
				}
				return true
			})
		}
		key := c.ClientIP()
		v, _ := buckets.LoadOrStore(key, &ipBucket{
			lim: rate.NewLimiter(rate.Limit(rps), burst),
		})
		b := v.(*ipBucket)
		b.last.Store(now)
		if !b.lim.Allow() {
			c.Abort()
			envelope.Render(c, nil, envelope.ErrRateLimited) // code 4290 → 429
			return
		}
		c.Next(ctx)
	}
}
