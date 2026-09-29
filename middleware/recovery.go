package middleware

import (
	"context"
	"runtime/debug"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
)

// Recovery panic 兜底：恢复后以 5001/500 渲染 envelope，error 日志（含 panic 值、path 与 stack 字段）
func Recovery(l logx.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				l.ErrorContext(ctx, "recovered panic",
					logx.Any("panic", r),
					logx.String("path", string(c.FullPath())),
					logx.String("stack", string(debug.Stack())))
				c.Abort()
				envelope.Render(c, nil, envelope.ErrPanic) // code 5001 → 500
			}
		}()
		c.Next(ctx)
	}
}
