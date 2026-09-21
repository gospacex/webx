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

// Otel 开 root span `<METHOD> <path>`；trace-id 经 envelope.WithRequestID 写入 hertz ctx，
// 并回写 `X-Request-Id` 响应头；span 经 trace.ContextWithSpan 传给下游 handler。
func Otel(tr trace.Tracer) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
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
