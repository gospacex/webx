package webx

import (
	"fmt"
	"os"
	"time"

	"github.com/gospacex/configx"
	"github.com/gospacex/logx"
)

// ======================= ConfigX 集成 =======================
//
// 与其余 x 系列 SDK 保持同一套入口三件套：
//
//	ReadConfig        只读配置，不装配（预检/兼容用）
//	ReadAndCreate     读配置 → 装配 App，返回可用句柄
//	MustReadAndCreate 同上，失败即退出进程
//
// 为什么需要单独的 WebSection 而不是直接 Unmarshal 进 Config：
// Config 是**装配**配置，含有不可序列化的成员——Logger/Tracer/Stats 是接口
// 或指针，ReadyCheck 与 Middlewares/preRun 是函数值。对它们做反序列化既无
// 意义（配置文件里不可能有函数），又会让 mapstructure 在遇到 YAML 里同名的
// 标量时报类型错误。因此这里用一个只含可序列化字段的 WebSection 承接配置，
// 再映射为 Config，把 Logger/Tracer 之类留给 FillDefaults 与 Option 决定。

// sectionKey webx 在配置文件中的段名。
const sectionKey = "webx"

// WebSection 配置文件中的 webx 段。
//
// tag 说明：三套 tag 齐备，分别对应 configx 的三条读取通道——yaml（本地文件
// 与文档示例）、json（远程配置中心下发）、mapstructure（本地 viper 通道）。
// 缺 mapstructure 会让多词字段（rate_limit_rps、health_prefix …）在本地
// 通道下静默解码为零值。
//
// 配置示例：
//
//	webx:
//	  addr: "0.0.0.0:8080"
//	  timeout: 5s
//	  rate_limit_rps: 100
//	  rate_limit_burst: 200
//	  cors_origins: ["https://app.example.com"]
//	  health_prefix: /healthz
//	  tls_cert_file: /etc/ssl/server.crt
//	  tls_key_file: /etc/ssl/server.key
//	  enable_alpn: true
//	  metrics_enabled: true
//	  otel:
//	    enabled: true
//	    exporter: otlp
//	    endpoint: http://localhost:4318
type WebSection struct {
	Addr           string        `yaml:"addr" json:"addr" mapstructure:"addr"`
	Timeout        time.Duration `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
	RateLimitRPS   int           `yaml:"rate_limit_rps" json:"rate_limit_rps" mapstructure:"rate_limit_rps"`
	RateLimitBurst int           `yaml:"rate_limit_burst" json:"rate_limit_burst" mapstructure:"rate_limit_burst"`
	CORSOrigins    []string      `yaml:"cors_origins" json:"cors_origins" mapstructure:"cors_origins"`
	HealthPrefix   string        `yaml:"health_prefix" json:"health_prefix" mapstructure:"health_prefix"`
	TLSCertFile    string        `yaml:"tls_cert_file" json:"tls_cert_file" mapstructure:"tls_cert_file"`
	TLSKeyFile     string        `yaml:"tls_key_file" json:"tls_key_file" mapstructure:"tls_key_file"`
	EnableALPN     bool          `yaml:"enable_alpn" json:"enable_alpn" mapstructure:"enable_alpn"`
	MetricsEnabled bool          `yaml:"metrics_enabled" json:"metrics_enabled" mapstructure:"metrics_enabled"`

	// OTEL 复用 logx 的 OTel 配置结构（调用链开关 + 导出目标）。
	// webx 自身不装配 TracerProvider，该开关的作用是把 webx 的链路状态
	// 登记进统一模型，见 Config.FillDefaults。
	OTEL logx.OTelConfig `yaml:"otel" json:"otel" mapstructure:"otel"`
}

// toConfig 把配置段映射为装配配置。
//
// 只搬运可序列化的字段：Logger/Tracer/Stats 缺省由 FillDefaults 补齐，
// ReadyCheck/Middlewares/preRun 只能由调用方经 Option 注入。
//
// 时间字段用 time.Duration（而非自定义类型）：configx 的本地通道是
// viper，其 mapstructure 钩子只识别 time.Duration 本身，写成自定义类型
// 会让 "5s" 这类写法解码失败。
func (s *WebSection) toConfig() Config {
	return Config{
		Addr:           s.Addr,
		TLSCertFile:    s.TLSCertFile,
		TLSKeyFile:     s.TLSKeyFile,
		EnableALPN:     s.EnableALPN,
		Timeout:        s.Timeout,
		RateLimitRPS:   s.RateLimitRPS,
		RateLimitBurst: s.RateLimitBurst,
		CORSOrigins:    s.CORSOrigins,
		HealthPrefix:   s.HealthPrefix,
		MetricsEnabled: s.MetricsEnabled,
		OTEL:           s.OTEL,
	}
}

// ReadConfigFromConfigx 从配置句柄取出 webx 段并转为装配配置。
//
// 需要注入 Middlewares / ReadyCheck / 自定义 Logger 的调用方用这个函数
// 拿到 Config，自行拼 Option 后再 New：
//
//	cfg, err := webx.ReadConfigFromConfigx(c)
//	app, err := webx.New(webx.WithConfig(cfg), webx.WithMiddleware(mw))
func ReadConfigFromConfigx(c configx.Config) (Config, error) {
	if c == nil {
		return Config{}, fmt.Errorf("webx: nil configx handle")
	}
	section := &WebSection{}
	if err := c.Unmarshal(sectionKey, section); err != nil {
		return Config{}, fmt.Errorf("webx: failed to unmarshal %s section: %w", sectionKey, err)
	}
	return section.toConfig(), nil
}

// ReadConfig 加载统一配置句柄（所有 x 系列 SDK 共享同一配置源）。
func ReadConfig(opts ...configx.ConfigOptions) error {
	_, err := configx.ReadConfig(opts...)
	return err
}

// ReadAndCreate 从 configx 读取 webx 配置并装配应用（不监听端口）。
//
// 监听由返回值的 Run() 触发，与 New 的行为一致。
func ReadAndCreate(opts ...configx.ConfigOptions) (*App, error) {
	c, err := configx.ReadConfig(opts...)
	if err != nil {
		return nil, fmt.Errorf("webx: failed to read config: %w", err)
	}
	cfg, err := ReadConfigFromConfigx(c)
	if err != nil {
		return nil, err
	}
	return New(WithConfig(cfg))
}

// MustReadAndCreate 类似 ReadAndCreate，失败时退出进程
func MustReadAndCreate(opts ...configx.ConfigOptions) *App {
	app, err := ReadAndCreate(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "webx: failed to assemble app: %v\n", err)
		os.Exit(1)
	}
	return app
}
