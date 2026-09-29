package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// CORS 默认预检白名单（未通过 options 自定义时的兜底值，
// 取代旧版「回显请求方法」的无白名单行为）
var (
	defaultAllowMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}
	defaultAllowHeaders = []string{"Content-Type", "Authorization", "X-Request-Id"}
	defaultPreflightAge = "600"
)

// CORSOpts CORS 可配置项：预检允许的方法/请求头白名单与缓存时长
type CORSOpts struct {
	// AllowMethods 预检允许的方法白名单；请求方法不在其中 → 预检 403。默认 GET/POST/PUT/DELETE/PATCH/OPTIONS
	AllowMethods []string
	// AllowHeaders 预检回显的允许请求头白名单。默认 Content-Type/Authorization/X-Request-Id
	AllowHeaders []string
	// MaxAge 预检结果缓存秒数（Access-Control-Max-Age）；空则默认 600
	MaxAge string
}

// CORSWith 白名单 origins + 可配置预检项（nil opts 等价于全默认）
func CORSWith(origins []string, opts *CORSOpts) app.HandlerFunc {
	return CORSWithOptions(origins, func(o *CORSOpts) {
		if opts != nil {
			if len(opts.AllowMethods) > 0 {
				o.AllowMethods = opts.AllowMethods
			}
			if len(opts.AllowHeaders) > 0 {
				o.AllowHeaders = opts.AllowHeaders
			}
			if opts.MaxAge != "" {
				o.MaxAge = opts.MaxAge
			}
		}
	})
}

// CORSWithOptions CORS 白名单 + option 函数配置（variadic，向后兼容扩展）
func CORSWithOptions(origins []string, fns ...func(*CORSOpts)) app.HandlerFunc {
	o := CORSOpts{
		AllowMethods: defaultAllowMethods,
		AllowHeaders: defaultAllowHeaders,
		MaxAge:       defaultPreflightAge,
	}
	for _, fn := range fns {
		fn(&o)
	}

	allow := make(map[string]struct{}, len(origins))
	for _, og := range origins {
		allow[og] = struct{}{}
	}
	methods := make(map[string]struct{}, len(o.AllowMethods))
	for _, m := range o.AllowMethods {
		methods[strings.ToUpper(strings.TrimSpace(m))] = struct{}{}
	}
	// 头白名单归一化为标准 Title-Case（http.Header 的键形态），比较时同样归一
	headers := make(map[string]struct{}, len(o.AllowHeaders))
	for _, h := range o.AllowHeaders {
		headers[httpHeaderNorm(h)] = struct{}{}
	}
	methodsStr := strings.ToUpper(strings.Join(o.AllowMethods, ", "))
	headersStr := strings.Join(o.AllowHeaders, ", ")

	return func(ctx context.Context, c *app.RequestContext) {
		origin := string(c.Request.Header.Peek("Origin"))
		if origin == "" || len(allow) == 0 {
			c.Next(ctx)
			return
		}
		if _, ok := allow[origin]; !ok {
			// 非白名单 origin：预检 403；简单请求不阻断（浏览器侧自行判）
			if isPreflight(c) {
				c.AbortWithStatus(consts.StatusForbidden)
			}
			return
		}
		// Vary：按 Origin 区分的响应不得被代理跨 origin 缓存复用
		setVary(c, "Origin")
		c.Response.Header.Set("Access-Control-Allow-Origin", origin)
		if isPreflight(c) {
			reqMethod := strings.ToUpper(string(c.Request.Header.Peek("Access-Control-Request-Method")))
			if reqMethod != "" {
				if _, ok := methods[reqMethod]; !ok {
					setVary(c, "Access-Control-Request-Method")
					c.AbortWithStatus(consts.StatusForbidden)
					return
				}
			}
			c.Response.Header.Set("Access-Control-Allow-Methods", methodsStr)
			c.Response.Header.Set("Access-Control-Allow-Headers", headersStr)
			if o.MaxAge != "" {
				c.Response.Header.Set("Access-Control-Max-Age", o.MaxAge)
			}
			c.AbortWithStatus(consts.StatusNoContent)
			return
		}
		c.Next(ctx)
	}
}

// CORS 按 origins 白名单放行；origins 为空 = 不启用 CORS（直接放行）。
// 预检（OPTIONS + Origin + Access-Control-Request-Method）：origin 命中且方法在默认
// 白名单内 → 204 + 固定允许头（不再回显请求方法）；origin 或方法未命中 → 403。
// 需要自定义方法/头白名单时用 CORSWith / CORSWithOptions。
// 非预检的简单请求不阻断（浏览器侧自行判）；无 Access-Control-Request-Method 的 OPTIONS
// 视为真实请求放行到路由（不抢占应用注册的 OPTIONS handler）。
func CORS(origins []string) app.HandlerFunc {
	return CORSWithOptions(origins)
}

// setVary 追加 Vary 头（已含该键则不重复；已有其他 Vary 值时逗号拼接）
func setVary(c *app.RequestContext, key string) {
	hv := c.Response.Header.Get("Vary")
	if strings.EqualFold(string(hv), key) {
		return
	}
	if len(hv) > 0 {
		key = strings.TrimSpace(string(hv)) + ", " + key
	}
	c.Response.Header.Set("Vary", key)
}

// httpHeaderNorm 头白名单归一化为标准 Title-Case（http.CanonicalHeaderKey），
// 与 http.Header 键形态一致，比较时双方同样归一
func httpHeaderNorm(h string) string {
	return http.CanonicalHeaderKey(strings.TrimSpace(h))
}

func isPreflight(c *app.RequestContext) bool {
	// OPTIONS + 预检特征头：普通 OPTIONS 真实请求不被 CORS 抢占
	return string(c.Method()) == "OPTIONS" &&
		len(c.Request.Header.Peek("Access-Control-Request-Method")) > 0
}
