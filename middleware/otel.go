package middleware

import (
	"context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Otel 开 root span `<METHOD> <path>`；入站 `X-Request-Id` 合法（32-hex）时种子化 trace-id
// （trace.ContextWithRemoteSpanContext），否则由 SDK 生成；trace-id 经
// envelope.WithRequestID 写入 hertz ctx，并回写 `X-Request-Id` 响应头。
func Otel(tr trace.Tracer) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 入站 X-Request-Id → 远程 span 种子（规格：客户端带头时贯穿，无头由服务端生成）
		ctx = seedInboundRequestID(ctx, c)
		name := fmt.Sprintf("%s %s", string(c.Method()), string(c.Path()))
		// v1.29.0 Go SDK: Start 返回 (Context, Span)——ctx 含活动 span
		ctx, span := tr.Start(ctx, name)
		defer span.End()
		span.SetAttributes(
			attribute.String("http.method", string(c.Method())),
			attribute.String("http.target", string(c.FullPath())),
		)
		rid := span.SpanContext().TraceID().String()
		envelope.WithRequestID(c, rid)
		c.Response.Header.Set("X-Request-Id", rid)
		c.Next(ctx)
		if c.Response.StatusCode() >= 500 {
			span.SetStatus(codes.Error, "server error")
		}
	}
}

// seedInboundRequestID 读入站 X-Request-Id：合法 32-hex trace-id 时构造 remote
// SpanContext 注入 ctx（span 名/属性仍本地生成，仅 trace-id 贯穿）；非法/缺失/全 0
// → 返回原 ctx（SDK 随机生成，不静默吞）。
func seedInboundRequestID(ctx context.Context, c *app.RequestContext) context.Context {
	inbound := string(c.Request.Header.Peek("X-Request-Id"))
	if inbound == "" {
		return ctx
	}
	tid, err := trace.TraceIDFromHex(inbound)
	if err != nil || !tid.IsValid() {
		return ctx
	}
	sid, err := trace.SpanIDFromHex("0100000000000001") // 占位本地 span id；SDK 会本地重新生成
	if err != nil {
		return ctx
	}
	rsc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		Remote:     true,
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithRemoteSpanContext(ctx, rsc)
}
