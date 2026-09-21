// handler 薄 handler：c.Param → logic → envelope.Render；不写业务
package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/logic"
	"go.opentelemetry.io/otel/trace"
)

// OrderHandler 订单 HTTP 入口
type OrderHandler struct {
	Logic  *logic.OrderLogic
	L      logx.Logger
	Tracer trace.Tracer
}

// New 构造
func New(l *logic.OrderLogic, gl logx.Logger, tr trace.Tracer) *OrderHandler {
	return &OrderHandler{Logic: l, L: gl, Tracer: tr}
}

// GetOrder GET /orders/:id
func (h *OrderHandler) GetOrder(ctx context.Context, c *app.RequestContext) {
	if h.Logic == nil { // fail fast（单测 nil 依赖契约）
		envelope.Render(c, nil, envelope.ErrBadRequest)
		return
	}
	ctx, span := h.Tracer.Start(ctx, "handler.GetOrder")
	defer span.End()
	data, err := h.Logic.Get(ctx, c.Param("id"))
	h.L.InfoContext(ctx, "handler.GetOrder: done", logx.String("id", c.Param("id")))
	envelope.Render(c, data, err)
}

// List GET /orders
func (h *OrderHandler) List(ctx context.Context, c *app.RequestContext) {
	if h.Logic == nil {
		envelope.Render(c, nil, envelope.ErrBadRequest)
		return
	}
	ctx, span := h.Tracer.Start(ctx, "handler.ListOrders")
	defer span.End()
	envelope.Render(c, []any{}, nil) // demo：List 经 logic 预留，此处返回空列表
}
