package envelope

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
)

// 覆盖率补充：Error.Error / Is 非 *Error 分支 / As 包装错误分支 / StatusOf 未映射兜底
func TestErrorTextAndIsBranches(t *testing.T) {
	e := Errorf(CodeNotFound, "订单 %d 不存在", 42)
	if got := e.Error(); got != "订单 42 不存在" {
		t.Fatalf("Error() = %q", got)
	}
	// Is：target 非 *Error → false
	if e.Is(errors.New("other")) {
		t.Fatal("Is(non-*Error) = true, want false")
	}
	// Is：同码不同实例 → true
	other := Errorf(CodeNotFound, "别的文案")
	if !e.Is(other) {
		t.Fatal("Is(same code) = false, want true")
	}
	if e.Is(ErrBadRequest) {
		t.Fatal("Is(different code) = true, want false")
	}
}

// As：包装过的 envelope 错误经 errors.As 提取；普通错误归一 CodeInternal
func TestAsWrappedAndPlain(t *testing.T) {
	base := Errorf(CodeConflict, "订单 1 已存在")
	wrapped := errors.Join(base, nil)
	if got := As(wrapped); got.Code != CodeConflict {
		t.Fatalf("As(wrapped) code = %d, want %d", got.Code, CodeConflict)
	}
	plain := As(errors.New("boom"))
	if plain.Code != CodeInternal {
		t.Fatalf("As(plain) code = %d, want %d", plain.Code, CodeInternal)
	}
	if As(nil) != nil {
		t.Fatal("As(nil) != nil")
	}
}

// StatusOf 未映射码兜底 500（RegisterCode 扩展后可改）
func TestStatusOfUnmappedFallsBack500(t *testing.T) {
	if got := StatusOf(2999); got != 500 {
		t.Fatalf("StatusOf(2999) = %d, want 500 fallback", got)
	}
	RegisterCode(2999, 410)
	if got := StatusOf(2999); got != 410 {
		t.Fatalf("StatusOf after RegisterCode = %d, want 410", got)
	}
}

// RequestIDFrom：已置位直接取；未置位生成 uuid 并回写（二次取同值）
func TestRequestIDFromWriteBack(t *testing.T) {
	c := app.NewContext(0)
	c.Params = param.Params{}
	// 未置位 → 生成 uuid 回写
	rid1 := RequestIDFrom(c)
	if rid1 == "" {
		t.Fatal("RequestIDFrom generated empty id")
	}
	// 二次取：命中 ctx 缓存
	if rid2 := RequestIDFrom(c); rid2 != rid1 {
		t.Fatalf("second call = %q, want same %q", rid2, rid1)
	}
	// 已置位非字符串/空值 → 重新生成
	WithRequestID(c, "")
	if RequestIDFrom(c) == rid1 {
		t.Fatal("empty stored id should regenerate, got same")
	}
	_ = context.Background()
}
