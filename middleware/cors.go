package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// CORS 按 origins 白名单放行；origins 为空 = 不启用 CORS（直接放行）。
// 预检（OPTIONS + Origin）：白名单命中 → 204 + 允许头；未命中 → 403。
func CORS(origins []string) app.HandlerFunc {
	allow := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allow[o] = struct{}{}
	}
	return func(ctx context.Context, c *app.RequestContext) {
		origin := string(c.Request.Header.Peek("Origin"))
		if origin == "" || len(allow) == 0 {
			c.Next(ctx)
			return
		}
		if _, ok := allow[origin]; !ok {
			if isPreflight(c) {
				c.AbortWithStatus(consts.StatusForbidden)
				return
			}
			c.Next(ctx) // 非预检的简单请求不阻断（浏览器侧自行判）
			return
		}
		c.Response.Header.Set("Access-Control-Allow-Origin", origin)
		if isPreflight(c) {
			if m := string(c.Request.Header.Peek("Access-Control-Request-Method")); m != "" {
				c.Response.Header.Set("Access-Control-Allow-Methods", m)
			}
			c.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
			c.Response.Header.Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(consts.StatusNoContent)
			return
		}
		c.Next(ctx)
	}
}

func isPreflight(c *app.RequestContext) bool {
	return string(c.Method()) == "OPTIONS"
}
