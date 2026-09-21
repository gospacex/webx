package middleware

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
)

func TestLoggingRecordsStats(t *testing.T) {
	stats := NewAccessStats()
	mw := Logging(logx.GetGlobal(), stats)
	c := app.NewContext(0)
	c.Request.SetMethod("POST")
	c.Request.SetRequestURI("/demo")
	c.SetHandlers(app.HandlersChain{mw, func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, "ok")
	}})
	c.Next(context.Background())

	snap := stats.Snapshot()
	if snap.Requests != 1 {
		t.Fatalf("stats.Requests = %d, want 1", snap.Requests)
	}
	if snap.LastStatus != 200 {
		t.Fatalf("stats.LastStatus = %d, want 200", snap.LastStatus)
	}
}
