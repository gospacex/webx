package middleware

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func newPreflightCtx(origin string) *app.RequestContext {
	c := app.NewContext(0)
	c.Request.SetMethod("OPTIONS")
	c.Request.SetRequestURI("/x")
	c.Request.SetHost("webx.test")
	if origin != "" {
		c.Request.Header.Set("Origin", origin)
		c.Request.Header.Set("Access-Control-Request-Method", "GET")
	}
	return c
}

func TestCORSAllowPreflight(t *testing.T) {
	mw := CORS([]string{"https://a.example"})
	c := newPreflightCtx("https://a.example")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())
	if got := string(c.Response.Header.Peek("Access-Control-Allow-Origin")); got != "https://a.example" {
		t.Fatalf("allow-origin = %q, want https://a.example", got)
	}
	if c.Response.StatusCode() != 204 {
		t.Fatalf("preflight status = %d, want 204", c.Response.StatusCode())
	}
}

func TestCORSRejectOrigin(t *testing.T) {
	mw := CORS([]string{"https://a.example"})
	c := newPreflightCtx("https://evil.example")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())
	if c.Response.StatusCode() != 403 {
		t.Fatalf("reject status = %d, want 403", c.Response.StatusCode())
	}
}

func TestCORSNoOriginPassThrough(t *testing.T) {
	mw := CORS([]string{"https://a.example"})
	c := newPreflightCtx("") // 无 Origin 头：非 CORS 请求，直接放行
	handled := false
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { handled = true }})
	c.Next(context.Background())
	if !handled {
		t.Fatal("handler not reached for non-CORS request")
	}
}

func TestCORSSimpleRequestRejectedOriginNotBlocked(t *testing.T) {
	// 白名单外 Origin 的简单（非预检）请求不阻断
	mw := CORS([]string{"https://a.example"})
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/x")
	c.Request.SetHost("webx.test")
	c.Request.Header.Set("Origin", "https://evil.example")
	handled := false
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { handled = true }})
	c.Next(context.Background())
	if !handled {
		t.Fatal("simple request should pass through to handler")
	}
	if got := string(c.Response.Header.Peek("Access-Control-Allow-Origin")); got != "" {
		t.Fatalf("allow-origin = %q, want empty for rejected origin", got)
	}
}

func TestCORSWithOptionsCustomMethodsAndMaxAge(t *testing.T) {
	// 自定义方法白名单 + MaxAge：预检响应头反映配置而非默认值
	mw := CORSWithOptions(
		[]string{"https://a.example"},
		func(o *CORSOpts) {
			o.AllowMethods = []string{"GET"}
			o.MaxAge = "123"
		},
	)
	c := app.NewContext(0)
	c.Request.SetMethod("OPTIONS")
	c.Request.SetRequestURI("/x")
	c.Request.SetHost("webx.test")
	c.Request.Header.Set("Origin", "https://a.example")
	c.Request.Header.Set("Access-Control-Request-Method", "GET")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())

	if c.Response.StatusCode() != 204 {
		t.Fatalf("preflight status = %d, want 204", c.Response.StatusCode())
	}
	if got := string(c.Response.Header.Peek("Access-Control-Allow-Methods")); got != "GET" {
		t.Fatalf("allow-methods = %q, want GET（自定义白名单）", got)
	}
	if got := string(c.Response.Header.Peek("Access-Control-Max-Age")); got != "123" {
		t.Fatalf("max-age = %q, want 123", got)
	}
}

func TestCORSWithOptionsCustomMethodRejected(t *testing.T) {
	// 请求方法不在自定义白名单内 → 预检 403
	mw := CORSWithOptions(
		[]string{"https://a.example"},
		func(o *CORSOpts) { o.AllowMethods = []string{"GET"} },
	)
	c := app.NewContext(0)
	c.Request.SetMethod("OPTIONS")
	c.Request.SetRequestURI("/x")
	c.Request.SetHost("webx.test")
	c.Request.Header.Set("Origin", "https://a.example")
	c.Request.Header.Set("Access-Control-Request-Method", "DELETE")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())
	if c.Response.StatusCode() != 403 {
		t.Fatalf("status = %d, want 403（方法不在白名单）", c.Response.StatusCode())
	}
}

func TestCORSWithOptionsCustomHeaders(t *testing.T) {
	// 自定义请求头白名单：预检回显配置的头而非默认值
	mw := CORSWithOptions(
		[]string{"https://a.example"},
		func(o *CORSOpts) { o.AllowHeaders = []string{"X-Custom-Token"} },
	)
	c := app.NewContext(0)
	c.Request.SetMethod("OPTIONS")
	c.Request.SetRequestURI("/x")
	c.Request.SetHost("webx.test")
	c.Request.Header.Set("Origin", "https://a.example")
	c.Request.Header.Set("Access-Control-Request-Method", "GET")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }})
	c.Next(context.Background())
	if got := string(c.Response.Header.Peek("Access-Control-Allow-Headers")); got != "X-Custom-Token" {
		t.Fatalf("allow-headers = %q, want X-Custom-Token", got)
	}
	if got := string(c.Response.Header.Peek("Vary")); !strings.Contains(got, "Origin") {
		t.Fatalf("Vary = %q, want 含 Origin", got)
	}
}

func TestCORSOPTIONSWithoutPreflightNotStolen(t *testing.T) {
	// 无 Access-Control-Request-Method 的 OPTIONS 是真实请求，不放行 204
	mw := CORS([]string{"https://a.example"})
	c := app.NewContext(0)
	c.Request.SetMethod("OPTIONS")
	c.Request.SetRequestURI("/x")
	c.Request.SetHost("webx.test")
	c.Request.Header.Set("Origin", "https://a.example")
	handled := false
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) { handled = true }})
	c.Next(context.Background())
	if !handled {
		t.Fatal("plain OPTIONS request should reach handler, not be answered as preflight")
	}
	if c.Response.StatusCode() == 204 {
		t.Fatal("plain OPTIONS must not get preflight 204")
	}
}
