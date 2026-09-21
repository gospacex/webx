package logic

import (
	"context"
	"testing"

	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"go.opentelemetry.io/otel"
)

// 契约：id 非法（解析失败或 <=0）→ 参数校验短路 1001，不触达 service
func TestLogicRejectsNonPositiveID(t *testing.T) {
	l := New(nil, logx.GetGlobal(), otel.GetTracerProvider().Tracer("t"))
	for _, raw := range []string{"0", "-3", "abc"} {
		_, err := l.Get(context.Background(), raw)
		if e := envelope.As(err); e.Code != envelope.CodeBadRequest {
			t.Fatalf("Get(%q) code = %d, want %d", raw, e.Code, envelope.CodeBadRequest)
		}
	}
}
