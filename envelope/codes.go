package envelope

import (
	"fmt"
	"sync"

	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// 分段错误码：1xxx 通用/数据、2xxx 业务（业务方扩展）、4xxx 网关语义、5xxx 服务端
const (
	CodeOK           = 0
	CodeBadRequest   = 1001
	CodeNotFound     = 1004
	CodeConflict     = 1009
	CodeBizRule      = 2000
	CodeUnauthorized = 4010
	CodeRateLimited  = 4290
	CodeInternal     = 5000
	CodePanic        = 5001
	CodeTimeout      = 5002
)

// 业务扩展码段：RegisterCode 仅允许落在该区间
const (
	codeBizExtMin = 2000
	codeBizExtMax = 2999
)

var (
	codeMu sync.RWMutex
	// codeToStatus 预置码 → HTTP 状态映射，业务方只能经 RegisterCode 扩展
	//（不导出：直接改写会绕过 codeMu，与热路径 StatusOf 并发读写构成 race）
	codeToStatus = map[int]int{
		CodeOK:           consts.StatusOK,
		CodeBadRequest:   consts.StatusBadRequest,
		CodeNotFound:     consts.StatusNotFound,
		CodeConflict:     consts.StatusConflict,
		CodeBizRule:      consts.StatusBadRequest,
		CodeUnauthorized: consts.StatusUnauthorized,
		CodeRateLimited:  consts.StatusTooManyRequests,
		CodeInternal:     consts.StatusInternalServerError,
		CodePanic:        consts.StatusInternalServerError,
		CodeTimeout:      consts.StatusGatewayTimeout,
	}
	// codeNames 预置码 → 稳定文案（init 后只读；sentinel 错误文案由此派生，单一来源）
	codeNames = map[int]string{
		CodeOK: "ok", CodeBadRequest: "bad request", CodeNotFound: "not found",
		CodeConflict: "conflict", CodeBizRule: "business rule violated",
		CodeUnauthorized: "unauthorized", CodeRateLimited: "rate limited",
		CodeInternal: "internal server error", CodePanic: "panic recovered",
		CodeTimeout: "request timeout",
	}
)

// RegisterCode 为业务扩展码注册 HTTP 状态映射。
// code 必须落在业务扩展段 [2000, 2999] 且不得覆盖预置码，
// httpStatus 必须是合法 HTTP 状态码 [100, 599]；违规直接 panic（同 Go 注册表惯例，如 regexp.MustCompile）。
func RegisterCode(code, httpStatus int) {
	codeMu.Lock()
	defer codeMu.Unlock()
	if code < codeBizExtMin || code > codeBizExtMax {
		panic(fmt.Sprintf("envelope: RegisterCode code %d 不在业务扩展段 [%d, %d]", code, codeBizExtMin, codeBizExtMax))
	}
	if _, preset := codeNames[code]; preset {
		panic(fmt.Sprintf("envelope: RegisterCode code %d 是预置码，禁止覆盖", code))
	}
	if httpStatus < 100 || httpStatus > 599 {
		panic(fmt.Sprintf("envelope: RegisterCode httpStatus %d 不在合法范围 [100, 599]", httpStatus))
	}
	codeToStatus[code] = httpStatus
}

// StatusOf 取 code 对应 HTTP 状态；未映射码兜底 500（不吞码：码仍出现在 envelope body）
func StatusOf(code int) int {
	codeMu.RLock()
	defer codeMu.RUnlock()
	if s, ok := codeToStatus[code]; ok {
		return s
	}
	return consts.StatusInternalServerError
}
