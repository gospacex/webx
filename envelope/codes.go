package envelope

import "sync"

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

var (
	codeMu     sync.RWMutex
	// CodeToStatus 预置码 → HTTP 状态映射，业务方可用 RegisterCode 扩展
	CodeToStatus = map[int]int{
		CodeOK:           200,
		CodeBadRequest:   400,
		CodeNotFound:     404,
		CodeConflict:     409,
		CodeBizRule:      400,
		CodeUnauthorized: 401,
		CodeRateLimited:  429,
		CodeInternal:     500,
		CodePanic:        500,
		CodeTimeout:      504,
	}
	// CodeNames 预置码 → 稳定文案（测试用码表遍历）
	CodeNames = map[int]string{
		CodeOK: "ok", CodeBadRequest: "bad request", CodeNotFound: "not found",
		CodeConflict: "conflict", CodeBizRule: "business rule violated",
		CodeUnauthorized: "unauthorized", CodeRateLimited: "rate limited",
		CodeInternal: "internal server error", CodePanic: "panic recovered",
		CodeTimeout: "request timeout",
	}
)

// RegisterCode 为业务扩展码（2xxx 段）注册 HTTP 状态映射
func RegisterCode(code, httpStatus int) {
	codeMu.Lock()
	defer codeMu.Unlock()
	CodeToStatus[code] = httpStatus
}

// StatusOf 取 code 对应 HTTP 状态；未映射码兜底 500（不吞码：码仍出现在 envelope body）
func StatusOf(code int) int {
	codeMu.RLock()
	defer codeMu.RUnlock()
	if s, ok := CodeToStatus[code]; ok {
		return s
	}
	return 500
}
