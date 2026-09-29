package webx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gospacex/configx"
)

// writeWebxConfig 落一份临时 config.yaml 并返回其路径。
func writeWebxConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return p
}

const webxSectionYAML = `
webx:
  addr: "127.0.0.1:18080"
  timeout: 3s
  rate_limit_rps: 50
  rate_limit_burst: 120
  cors_origins:
    - "https://app.example.com"
    - "https://admin.example.com"
  health_prefix: /healthz
  enable_alpn: true
  metrics_enabled: true
  otel:
    enabled: true
    exporter: otlp
    service_name: webx-entry-test
`

// TestReadConfigFromConfigx_MapsEveryField 守住「字段静默归零」：WebSection
// 缺 mapstructure tag 时，rate_limit_rps / health_prefix 这类多词字段会在
// 本地 viper 通道下解成零值，而 FillDefaults 又把零值补成默认值——表现为
// 配置写了不生效。删掉结构体上的 tag，本用例必须失败。
func TestReadConfigFromConfigx_MapsEveryField(t *testing.T) {
	c, err := configx.ReadConfig(configx.ConfigOptions{ConfigFile: writeWebxConfig(t, webxSectionYAML)})
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	cfg, err := ReadConfigFromConfigx(c)
	if err != nil {
		t.Fatalf("ReadConfigFromConfigx: %v", err)
	}

	if cfg.Addr != "127.0.0.1:18080" {
		t.Errorf("Addr = %q, want 127.0.0.1:18080", cfg.Addr)
	}
	if cfg.Timeout != 3*time.Second {
		t.Errorf("Timeout = %v, want 3s", cfg.Timeout)
	}
	if cfg.RateLimitRPS != 50 {
		t.Errorf("RateLimitRPS = %d, want 50", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 120 {
		t.Errorf("RateLimitBurst = %d, want 120", cfg.RateLimitBurst)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "https://app.example.com" {
		t.Errorf("CORSOrigins = %v, want two entries", cfg.CORSOrigins)
	}
	if cfg.HealthPrefix != "/healthz" {
		t.Errorf("HealthPrefix = %q, want /healthz", cfg.HealthPrefix)
	}
	if !cfg.EnableALPN {
		t.Error("EnableALPN = false, want true")
	}
	if !cfg.MetricsEnabled {
		t.Errorf("MetricsEnabled = false, want true")
	}
	if cfg.OTEL.ServiceName != "webx-entry-test" {
		t.Errorf("OTEL.ServiceName = %q, want webx-entry-test", cfg.OTEL.ServiceName)
	}
}

// 不可序列化的成员必须留空，由 FillDefaults / Option 决定：
// 否则 New 会拿到 nil Logger 之类的半成品配置。
func TestReadConfigFromConfigx_LeavesRuntimeFieldsUnset(t *testing.T) {
	c, err := configx.ReadConfig(configx.ConfigOptions{ConfigFile: writeWebxConfig(t, webxSectionYAML)})
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	cfg, err := ReadConfigFromConfigx(c)
	if err != nil {
		t.Fatalf("ReadConfigFromConfigx: %v", err)
	}
	if cfg.Logger != nil {
		t.Error("Logger must stay nil so FillDefaults supplies the global logger")
	}
	if cfg.Tracer != nil {
		t.Error("Tracer must stay nil so FillDefaults supplies the global tracer")
	}
	if cfg.Stats != nil {
		t.Error("Stats must stay nil so FillDefaults supplies a fresh AccessStats")
	}
	if cfg.ReadyCheck != nil || cfg.Middlewares != nil || cfg.preRun != nil {
		t.Error("function-valued fields must not be populated from config")
	}
}

// ReadAndCreate 装配完整应用：段的字段生效，且 FillDefaults 补上了运行时缺省。
func TestReadAndCreate_AssemblesApp(t *testing.T) {
	app, err := ReadAndCreate(configx.ConfigOptions{ConfigFile: writeWebxConfig(t, webxSectionYAML)})
	if err != nil {
		t.Fatalf("ReadAndCreate: %v", err)
	}
	if app == nil {
		t.Fatal("ReadAndCreate returned nil app")
	}
	got := app.Config()
	if got.Addr != "127.0.0.1:18080" {
		t.Errorf("Addr = %q, want 127.0.0.1:18080", got.Addr)
	}
	if got.RateLimitRPS != 50 {
		t.Errorf("RateLimitRPS = %d, want 50", got.RateLimitRPS)
	}
	// FillDefaults 之后这些不该再是零值。
	if got.Logger == nil {
		t.Error("Logger = nil after assemble")
	}
	if got.Tracer == nil {
		t.Error("Tracer = nil after assemble")
	}
	if got.Stats == nil {
		t.Error("Stats = nil after assemble")
	}
}

// 段缺失必须报错：静默装配一个全默认应用会让调用方以为配置生效了。
func TestReadAndCreate_MissingSectionFails(t *testing.T) {
	app, err := ReadAndCreate(configx.ConfigOptions{
		ConfigFile: writeWebxConfig(t, "logx:\n  level: info\n"),
	})
	if err == nil {
		t.Fatal("ReadAndCreate with no webx section must fail")
	}
	if !strings.Contains(err.Error(), "webx") {
		t.Errorf("error %q should name the webx section", err.Error())
	}
	if app != nil {
		t.Error("app must be nil when ReadAndCreate fails")
	}
}

// 非法配置必须由 Validate 拦住，而不是装配出一个监听不了的应用。
func TestReadAndCreate_InvalidAddrFails(t *testing.T) {
	_, err := ReadAndCreate(configx.ConfigOptions{
		ConfigFile: writeWebxConfig(t, "webx:\n  addr: \"not-a-hostport\"\n"),
	})
	if err == nil {
		t.Fatal("ReadAndCreate must reject a malformed addr")
	}
}

// nil 句柄不应 panic。
func TestReadConfigFromConfigx_NilHandle(t *testing.T) {
	if _, err := ReadConfigFromConfigx(nil); err == nil {
		t.Fatal("ReadConfigFromConfigx(nil) must fail")
	}
}

func TestReadConfig_DelegatesWithoutPanic(t *testing.T) {
	if err := ReadConfig(configx.ConfigOptions{ConfigFile: writeWebxConfig(t, webxSectionYAML)}); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
}

func TestMustReadAndCreate_ReturnsApp(t *testing.T) {
	if app := MustReadAndCreate(configx.ConfigOptions{ConfigFile: writeWebxConfig(t, webxSectionYAML)}); app == nil {
		t.Fatal("MustReadAndCreate returned nil")
	}
}
