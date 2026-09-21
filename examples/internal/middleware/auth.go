// middleware 业务鉴权（复用 envelope 4010/401）
package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/webx/envelope"
)

// Auth 校验 X-Token；不匹配 → envelope 4010 + Abort（业务中间件，经 WithMiddleware 追加链尾）
func Auth(token string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if string(c.Request.Header.Peek("X-Token")) != token {
			envelope.Render(c, nil, envelope.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}
