package webx

import (
	"context"
	"fmt"
	"github.com/gospacex/traceflag"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Config webx 装配配置（缺省值经 FillDefaults 补齐）
type Config struct {
	Addr           string // "host:port"，默认 "0.0.0.0:8080"
	Logger         logx.Logger
	TLSCertFile    string
	TLSKeyFile     string
	EnableALPN     bool
	Timeout        time.Duration
	RateLimitRPS   int
	RateLimitBurst int
	CORSOrigins    []string
	HealthPrefix   string
	ReadyCheck     func(context.Context) error
	Tracer         trace.Tracer
	Stats          *middleware.AccessStats
	MetricsEnabled bool
	OTEL           logx.OTelConfig      // 记录用；TracerProvider 由 cmd 层经 logx 装配
	MeterProvider  metric.MeterProvider // Task 5 接线
	Middlewares    []app.HandlerFunc
	preRun         []func(*App) // 非导出：TestApp 启动监听前回调（路由注册）
}

// FillDefaults 对零值字段补齐缺省
func (c *Config) FillDefaults() {
	if c.Addr == "" {
		c.Addr = "0.0.0.0:8080"
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	if c.HealthPrefix == "" {
		c.HealthPrefix = "/healthz"
	}
	if c.Logger == nil {
		c.Logger = logx.GetGlobal()
	}
	if c.Tracer == nil {
		c.Tracer = otel.GetTracerProvider().Tracer("webx") // 全局缺省 noop delegate
	}
	if c.Stats == nil {
		c.Stats = middleware.NewAccessStats()
	}
	// 接入 hubx 统一调用链开关。注意 webx 自身不装配 TracerProvider
	// （由 cmd 层经 logx 装配，见下方 OTEL 字段注释），所以这里的作用是
	// 把 webx 的开关状态登记进统一模型，并让全局总闸能被查询到。
	// 真正决定 HTTP 中间件埋不埋点的是 Config.Tracer 是否为 noop。
	c.OTEL.Enabled = traceflag.Apply("webx", c.OTEL.Enabled)

	if c.OTEL.ServiceName == "" {
		c.OTEL.ServiceName = "webx"
	}
}

// Validate 校验配置合法性（New / TestApp 在 FillDefaults 后调用）
func (c *Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("webx: Addr 不能为空")
	}
	_, port, err := net.SplitHostPort(c.Addr)
	if err != nil {
		return fmt.Errorf("webx: Addr %q 非 host:port 形式: %w", c.Addr, err)
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("webx: Addr %q 端口非法（须 1..65535）", c.Addr)
	}
	if c.HealthPrefix == "" {
		return fmt.Errorf("webx: HealthPrefix 不能为空（FillDefaults 后不应出现）")
	}
	if !strings.HasPrefix(c.HealthPrefix, "/") || c.HealthPrefix == "/" {
		return fmt.Errorf("webx: HealthPrefix %q 须以 / 开头且不能为裸 /", c.HealthPrefix)
	}
	if c.RateLimitRPS < 0 {
		return fmt.Errorf("webx: RateLimitRPS 不能为负")
	}
	if c.RateLimitBurst < 0 {
		return fmt.Errorf("webx: RateLimitBurst 不能为负")
	}
	return nil
}

// Option 配置选项
type Option func(*Config)

// WithConfig 整体注入配置（后续 Option 可覆盖其字段）
func WithConfig(cfg Config) Option {
	return func(c *Config) { *c = cfg }
}

// WithLogger 注入 logx logger
func WithLogger(l logx.Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithTLS 证书文件 + ALPN（H2 尽力协商，h1-over-TLS 兜底）
func WithTLS(certFile, keyFile string, alpn bool) Option {
	return func(c *Config) {
		c.TLSCertFile = certFile
		c.TLSKeyFile = keyFile
		c.EnableALPN = alpn
	}
}

// WithMiddleware 业务中间件，追加于默认链链尾（对应 3.4）
func WithMiddleware(mws ...app.HandlerFunc) Option {
	return func(c *Config) { c.Middlewares = append(c.Middlewares, mws...) }
}

// WithReadyCheck ready 探针自定义（nil/成功 → 200，错误 → 503）
func WithReadyCheck(f func(context.Context) error) Option {
	return func(c *Config) { c.ReadyCheck = f }
}

// WithOTLP 记录 OTel 导出配置（TracerProvider 实际装配在 cmd 层经 logx）
func WithOTLP(oc logx.OTelConfig) Option {
	return func(c *Config) { c.OTEL = oc }
}

// WithMetrics 启用 otel/metric instrument（BindMeter，Task 5）
func WithMetrics(enabled bool) Option {
	return func(c *Config) { c.MetricsEnabled = enabled }
}

// WithTracer 注入真实 Tracer（测试用 InMemory exporter）
func WithTracer(tr trace.Tracer) Option {
	return func(c *Config) { c.Tracer = tr }
}

// WithMeterProvider 注入 otel/metric Provider（MetricsEnabled=true 时 BindMeter，Task 5）
func WithMeterProvider(p metric.MeterProvider) Option {
	return func(c *Config) { c.MeterProvider = p }
}

// WithPreRun 在 New / TestApp 启动监听前回调（业务路由注册须早于 Run，避免 hertz 路由树并发写）
func WithPreRun(fn func(a *App)) Option {
	return func(c *Config) { c.preRun = append(c.preRun, fn) }
}
