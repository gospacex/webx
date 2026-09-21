// routes 路由注册源唯一（手写）。
//
// hz 生成区约定：重跑 hz 生成器仅影响 app/ 下的生成物（handler stub / auto 路由），
// internal/ 下手写代码（含业务中间件）不被生成器触碰；本骨架不跑生成器，
// 故 app/ 全部为手写，routes.go 即注册源。
package app

import (
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gospacex/webx/examples/app/handler"
)

// RegisterRoutes 注册 demo 路由（调用链入链由 webx 默认链的 otel 中间件开 root span
// `<METHOD> <path>`，各层子 span 在 handler/logic/service 内开）
func RegisterRoutes(e *route.Engine, h *handler.OrderHandler) {
	e.GET("/orders", h.List)
	e.GET("/orders/:id", h.GetOrder)
}
