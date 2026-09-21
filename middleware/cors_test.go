package middleware

import (
	"context"
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
