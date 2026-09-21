package webx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/middleware"
	"go.opentelemetry.io/otel"
)

// App webx 应用（内嵌 *server.Hertz；TestApp 独立实例，不触碰单例）
type App struct {
	*server.Hertz
	cfg   Config
	ln    net.Listener
	stats *middleware.AccessStats
}

// New 按选项装配默认链 + health + TLS；不监听（监听由 Run 或 TestApp 触发）
func New(opts ...Option) (*App, error) {
	cfg := Config{}
	for _, o := range opts {
		o(&cfg)
	}
	cfg.FillDefaults()

	srvOpts, err := buildServerOptions(&cfg, nil)
	if err != nil {
		return nil, err
	}
	h := server.New(srvOpts...)

	// 默认链 → 业务中间件追加链尾（3.4）
	h.Engine.Handlers = append(
		middleware.DefaultChain(cfg.Tracer, cfg.Stats, cfg.Logger,
			cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.CORSOrigins, cfg.Timeout),
		cfg.Middlewares...,
	)
	registerHealth(h.Engine, &cfg)
	bindMetrics(&cfg)

	return &App{Hertz: h, cfg: cfg, stats: cfg.Stats}, nil
}

// buildServerOptions 组装 hertz options；ln 非 nil（TestApp）时走 listener 分支。
// 实测：server.New 接受 pkg/common/config.Option（pkg/app/server/hertz.go:39）。
func buildServerOptions(cfg *Config, ln net.Listener) ([]config.Option, error) {
	if ln != nil {
		opts := []config.Option{
			server.WithListener(ln),
			server.WithTransport(standard.NewTransporter),
		}
		if cfg.TLSCertFile != "" {
			tlsCfg, err := loadTLSCfg(cfg)
			if err != nil {
				return nil, err
			}
			opts = append(opts, server.WithTLS(tlsCfg))
			if cfg.EnableALPN {
				opts = append(opts, server.WithALPN(true))
			}
		}
		return opts, nil
	}
	if cfg.TLSCertFile != "" {
		tlsCfg, err := loadTLSCfg(cfg)
		if err != nil {
			return nil, err
		}
		opts := []config.Option{
			server.WithTLS(tlsCfg),
			server.WithTransport(standard.NewTransporter), // TLS 切 standard transporter
			server.WithHostPorts(cfg.Addr),
		}
		if cfg.EnableALPN {
			opts = append(opts, server.WithALPN(true))
		}
		return opts, nil
	}
	return []config.Option{server.WithHostPorts(cfg.Addr)}, nil
}

// loadTLSCfg 证书加载 fail fast：错误信息含两个文件路径
func loadTLSCfg(cfg *Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("webx: load TLS cert/key (%s, %s): %w", cfg.TLSCertFile, cfg.TLSKeyFile, err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

// registerHealth /healthz/live + /healthz/ready
func registerHealth(e *route.Engine, cfg *Config) {
	e.GET(cfg.HealthPrefix+"/live", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]any{"status": "alive"})
	})
	e.GET(cfg.HealthPrefix+"/ready", func(ctx context.Context, c *app.RequestContext) {
		if cfg.ReadyCheck == nil || cfg.ReadyCheck(ctx) == nil {
			c.JSON(200, map[string]any{"status": "ready"})
			return
		}
		c.JSON(503, map[string]any{"status": "unavailable"})
	})
}

// bindMetrics MetricsEnabled 时接线 otel/metric instrument（失败仅告警，不阻断服务）
func bindMetrics(cfg *Config) {
	if !cfg.MetricsEnabled {
		return
	}
	provider := cfg.MeterProvider
	if provider == nil {
		provider = otel.GetMeterProvider() // 缺省全局（noop 安全）
	}
	if err := cfg.Stats.BindMeter(provider.Meter("webx")); err != nil {
		cfg.Logger.Warn("webx: bind metric instruments failed", logx.Err(err))
	}
}

// Config 只读导出当前配置（测试断言缺省值用）
func (a *App) Config() Config { return a.cfg }

// URL 取址（TLS → https）；TestApp（有 listener）返回实际随机端口
func (a *App) URL() string {
	scheme := "http"
	if a.cfg.TLSCertFile != "" {
		scheme = "https"
	}
	if a.ln != nil {
		return scheme + "://" + a.ln.Addr().String()
	}
	return scheme + "://" + a.cfg.Addr
}

// AppSingleton 单例：首次默认配置 New 后缓存；构造失败缓存 error（不静默吞，logx 记录）
var (
	singletonOnce sync.Once
	singletonApp  *App
	singletonErr  error
)

func AppSingleton() (*App, error) {
	singletonOnce.Do(func() {
		singletonApp, singletonErr = New()
		if singletonErr != nil {
			logx.Error("webx: singleton construction failed", logx.Err(singletonErr))
		}
	})
	return singletonApp, singletonErr
}

// TestApp 独立实例：127.0.0.1:0 随机端口 + 后台 Run + 等待 IsRunning（3s 超时）
func TestApp(opts ...Option) (*App, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("webx: test listen: %w", err)
	}
	cfg := Config{}
	for _, o := range opts {
		o(&cfg)
	}
	cfg.FillDefaults()

	srvOpts, err := buildServerOptions(&cfg, ln)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	h := server.New(srvOpts...)
	h.Engine.Handlers = append(
		middleware.DefaultChain(cfg.Tracer, cfg.Stats, cfg.Logger,
			cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.CORSOrigins, cfg.Timeout),
		cfg.Middlewares...,
	)
	registerHealth(h.Engine, &cfg)
	bindMetrics(&cfg)

	a := &App{Hertz: h, cfg: cfg, ln: ln, stats: cfg.Stats}
	go h.Run()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if h.IsRunning() {
			return a, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, errors.New("webx: test server not running after 3s")
}

// Run 阻塞监听（生产入口）
func (a *App) Run() error {
	return a.Hertz.Run()
}

// Shutdown 优雅停机：OnShutdown 钩子 → 排空 in-flight → transporter 停
func (a *App) Shutdown(ctx context.Context) error {
	return a.Hertz.Shutdown(ctx)
}
