// trace 调用链开关装配（examples/internal/trace）：
// 开启 → logx OTel.Enabled + SetupTracerProvider（分层路由入链经 logx 实现）；
// 关闭 → noop Tracer，行为不变
package trace

import (
	"github.com/gospacex/logx"
	logxzap "github.com/gospacex/logx/zap"
	"github.com/gospacex/webx/examples/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// BuildTracer 按开关返回（Tracer, 是否启用调用链）
func BuildTracer(cfg *config.Config) (trace.Tracer, bool, error) {
	if !cfg.Trace.Enabled {
		return otel.GetTracerProvider().Tracer("webx-demo"), false, nil
	}
	otCfg := logx.OTelConfig{
		Enabled:     true,
		Exporter:    cfg.Trace.Exporter,
		ServiceName: "webx-demo",
	}
	if _, err := logxzap.SetupTracerProvider(otCfg); err != nil {
		return nil, false, err
	}
	// 全局 TracerProvider 已被 logx 设置（otel.SetTracerProvider + TextMapPropagator）；
	// 各层子 span 与日志入链均由 ctx 中活动 span 驱动，cmd 层取全局句柄注入即可
	return otel.GetTracerProvider().Tracer("webx-demo"), true, nil
}
