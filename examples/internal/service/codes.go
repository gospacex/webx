// service 业务错误码（2xxx 段扩展，经 envelope.RegisterCode 映射 HTTP 状态）
package service

import "github.com/gospacex/webx/envelope"

// 业务码（2xxx 段）
const (
	CodeOrderArchived = 2001
)

func init() {
	// 归档订单不可再取 → 骨架用 400 兜底
	envelope.RegisterCode(CodeOrderArchived, 400)
}
