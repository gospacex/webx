package envelope

import (
	"errors"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
)

// Response 统一响应封装
type Response struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

// Error 分段错误：code + message，支持 errors.Is 按码判定
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }

// Is 按错误码等价判定（1004 的所有实例 Is(ErrNotFound) 为 true）
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// 预设 sentinel 错误
var (
	ErrBadRequest   = &Error{Code: CodeBadRequest, Message: "bad request"}
	ErrNotFound     = &Error{Code: CodeNotFound, Message: "not found"}
	ErrConflict     = &Error{Code: CodeConflict, Message: "conflict"}
	ErrBizRule      = &Error{Code: CodeBizRule, Message: "business rule violated"}
	ErrUnauthorized = &Error{Code: CodeUnauthorized, Message: "unauthorized"}
	ErrRateLimited  = &Error{Code: CodeRateLimited, Message: "rate limited"}
	ErrInternal     = &Error{Code: CodeInternal, Message: "internal server error"}
	ErrPanic        = &Error{Code: CodePanic, Message: "panic recovered"}
	ErrTimeout      = &Error{Code: CodeTimeout, Message: "request timeout"}
)

// Errorf 创建带格式化文案的 *Error
func Errorf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// As 将任意 error 归一为 *Error（非 envelope 错误 → CodeInternal，不吞错）
func As(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: CodeInternal, Message: err.Error()}
}

// requestIDKey 是 hertz ctx.Keys 中 request-id 的键（*app.RequestContext 非 context.Context，
// 只能经 c.Set/c.Get 流转，见「API 校正记录」）
const requestIDKey = "webx.request_id"

// WithRequestID 将 request id 写入 hertz ctx（由 otel 中间件在读入 X-Request-Id 后调用）
func WithRequestID(c *app.RequestContext, id string) {
	c.Set(requestIDKey, id)
}

// RequestIDFrom 从 hertz ctx 取 request id；无则生成 uuid（保证每条路径可追踪）
func RequestIDFrom(c *app.RequestContext) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	rid := uuid.NewString()
	WithRequestID(c, rid) // 回写：同请求后续中间件/handler 取到同一 id
	return rid
}

// Render 统一渲染 JSON envelope：err 为 nil → code=0 + data；
// err 非 nil → code/message 取自错误、data 为 nil、HTTP 状态按 CodeToStatus 映射
func Render(c *app.RequestContext, data any, err error) {
	rid := RequestIDFrom(c)
	if err == nil {
		c.JSON(consts.StatusOK, Response{Code: CodeOK, Message: "ok", Data: data, RequestID: rid})
		return
	}
	e := As(err)
	c.JSON(StatusOf(e.Code), Response{Code: e.Code, Message: e.Message, Data: nil, RequestID: rid})
}
