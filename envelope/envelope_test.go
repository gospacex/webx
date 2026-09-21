package envelope

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// newTestContext 造一个可用的 *app.RequestContext（hertz v0.10.6 无 NewRequestContext，
// 用 NewContext(0) + 手工填充 Request 字段；Response 为内嵌值，直接读）。
func newTestContext(requestID string) *app.RequestContext {
	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("/demo/42")
	c.Request.SetHost("webx.test")
	if requestID != "" {
		WithRequestID(c, requestID)
	}
	return c
}

func TestRenderSuccess(t *testing.T) {
	c := newTestContext("rid-42")
	Render(c, map[string]string{"k": "v"}, nil)

	if got := c.Response.StatusCode(); got != consts.StatusOK {
		t.Fatalf("status = %d, want %d", got, consts.StatusOK)
	}
	if !strings.Contains(string(c.Response.Header.ContentType()), "application/json") {
		t.Fatalf("content-type = %s, want application/json", c.Response.Header.ContentType())
	}
	var r Response
	if err := json.Unmarshal(c.Response.Body(), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Code != CodeOK || r.Message != "ok" || r.RequestID != "rid-42" || r.Data == nil {
		t.Fatalf("unexpected success response: %+v", r)
	}
}

func TestRenderFailure(t *testing.T) {
	c := newTestContext("rid-42")
	Render(c, nil, Errorf(CodeNotFound, "用户 %d 不存在", 42))

	if got := c.Response.StatusCode(); got != consts.StatusNotFound {
		t.Fatalf("status = %d, want 404", got)
	}
	var r Response
	if err := json.Unmarshal(c.Response.Body(), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Code != CodeNotFound || !strings.Contains(r.Message, "42") || r.RequestID != "rid-42" || r.Data != nil {
		t.Fatalf("unexpected failure response: %+v", r)
	}
}

func TestErrorsIs(t *testing.T) {
	err := Errorf(CodeNotFound, "user %d missing", 7)
	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is(code=1004, ErrNotFound) = false, want true")
	}
	if errors.Is(err, ErrBadRequest) {
		t.Error("errors.Is(code=1004, ErrBadRequest) = true, want false")
	}
	// 非 envelope 错误归一为 CodeInternal
	if e := As(errors.New("boom")); e.Code != CodeInternal {
		t.Fatalf("As(plain) code = %d, want %d", e.Code, CodeInternal)
	}
}

func TestCodeTableComplete(t *testing.T) {
	for code, name := range CodeNames {
		if _, ok := CodeToStatus[code]; !ok {
			t.Errorf("CodeToStatus missing preset code %d (%s)", code, name)
		}
		if StatusOf(code) == 0 {
			t.Errorf("StatusOf(%d) = 0, want mapped status", code)
		}
	}
}

func TestRegisterCode(t *testing.T) {
	RegisterCode(2001, 404)
	if got := StatusOf(2001); got != 404 {
		t.Fatalf("StatusOf(2001) = %d, want 404", got)
	}
}
