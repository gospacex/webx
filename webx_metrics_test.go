package webx

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

// metricsBody 采集口固定输出，便于逐字节比对。
const metricsBody = "webx_requests_total 3\n"

// okMetricsHandler 一个"看起来像 promhttp"的采集口。
func okMetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(metricsBody))
	})
}

// TestMetricsEndpointServed 装上采集口后，缺省前缀 /metrics 可访问。
func TestMetricsEndpointServed(t *testing.T) {
	a, err := TestApp(WithMetricsEndpoint(okMetricsHandler()))
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	if got := a.Config().MetricsPrefix; got != DefaultMetricsPrefix {
		t.Fatalf("MetricsPrefix = %q, want %q", got, DefaultMetricsPrefix)
	}
	code, body, err := getURL(t, a.URL()+DefaultMetricsPrefix)
	if err != nil {
		t.Fatalf("GET %s: %v", DefaultMetricsPrefix, err)
	}
	if code != 200 {
		t.Fatalf("GET %s = %d, want 200", DefaultMetricsPrefix, code)
	}
	if !strings.Contains(string(body), "webx_requests_total") {
		t.Fatalf("采集口输出未透传：%q", body)
	}
}

// TestMetricsEndpointAbsent 未装采集口时不注册该路由（presence-based）。
//
// 断言"不是 200"而不是"一定是 404"：路由未注册时 hertz 的缺省响应由框架决定，
// 本用例要钉住的是「没配就没有」，不是在钉框架的状态码。
func TestMetricsEndpointAbsent(t *testing.T) {
	a, err := TestApp()
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	if a.Config().MetricsHandler != nil {
		t.Fatal("未提供采集口时 Config.MetricsHandler 必须为空")
	}
	code, _, err := getURL(t, a.URL()+DefaultMetricsPrefix)
	if err != nil {
		t.Fatalf("GET %s: %v", DefaultMetricsPrefix, err)
	}
	if code == 200 {
		t.Fatalf("未装采集口时 %s 不应有内容（code=%d）", DefaultMetricsPrefix, code)
	}
}

// TestMetricsEndpointCustomPrefixAndRateLimitExempt 自定义前缀 + 采集口豁免入口限流。
//
// 豁免是**必须**的：采集器按固定周期拉取，被限流拒掉的表现是面板数据莫名缺一段，
// 且不产生任何错误日志。反向探针：把 registerMetrics 的 exempt 追加去掉，
// 本用例的采集口连打就会拿到 429。
//
// 同时用 /ping 做对照组：证明限流器确实是开着的 —— 否则"采集口都是 200"可能只是
// 因为限流压根没生效，用例会永远绿。
func TestMetricsEndpointCustomPrefixAndRateLimitExempt(t *testing.T) {
	const prefix = "/internal/metrics"
	a, err := TestApp(
		// 注意顺序：WithConfig 是 `*c = cfg` 的**整体覆盖**，排在它前面的 Option
		// （MetricsHandler / preRun）会被整个抹掉 —— 症状是路由 404 而编译不报错。
		// 故它必须排第一位，后续 Option 只做字段覆盖。
		WithConfig(Config{
			// TestApp 走 listener 分支，地址不参与监听；这里只为了过 Validate
			//（它会拒掉 ":0" 这种端口，故给一个合法值）。
			Addr:           "127.0.0.1:18099",
			RateLimitRPS:   1,
			RateLimitBurst: 1,
			MetricsHandler: okMetricsHandler(),
			MetricsPrefix:  prefix,
		}),
		WithPreRun(func(a *App) {
			// 与既有用例一致走 a.Engine.GET：注册必须落在 Run 用的同一个引擎上
			a.Engine.GET("/ping", func(_ context.Context, c *app.RequestContext) { c.String(200, "pong") })
		}),
	)
	if err != nil {
		t.Fatalf("TestApp: %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	for i := 0; i < 3; i++ {
		code, body, err := getURL(t, a.URL()+prefix)
		if err != nil {
			t.Fatalf("第 %d 次 GET %s: %v", i+1, prefix, err)
		}
		if code != 200 {
			t.Fatalf("第 %d 次 GET %s = %d，采集口必须豁免入口限流", i+1, prefix, code)
		}
		if !strings.Contains(string(body), "webx_requests_total") {
			t.Fatalf("第 %d 次响应体不含指标：%q", i+1, body)
		}
	}

	// 对照组：普通路由在同一限流器下第二次必被拒。
	if code, _, err := getURL(t, a.URL()+"/ping"); err != nil || code != 200 {
		t.Fatalf("首次 /ping = %d (err=%v)，对照组前提不成立", code, err)
	}
	if code, _, err := getURL(t, a.URL()+"/ping"); err != nil {
		t.Fatalf("二次 /ping: %v", err)
	} else if code != 429 {
		t.Fatalf("二次 /ping = %d，want 429 —— 限流器没生效，本用例的豁免断言也就没有意义", code)
	}
}
