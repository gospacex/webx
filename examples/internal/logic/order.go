// logic 薄适配：参数校验/转换 + 调 service；只透传错误
package logic

import (
	"context"
	"strconv"

	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/service"
	"go.opentelemetry.io/otel/trace"
)

// OrderLogic 订单逻辑层
type OrderLogic struct {
	Svc    *service.OrderService
	L      logx.Logger
	Tracer trace.Tracer
}

// New 构造
func New(svc *service.OrderService, l logx.Logger, tr trace.Tracer) *OrderLogic {
	return &OrderLogic{Svc: svc, L: l, Tracer: tr}
}

// Get 参数校验（id 必须为正整数）→ 调 service
func (g *OrderLogic) Get(ctx context.Context, rawID string) (any, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return nil, envelope.Errorf(envelope.CodeBadRequest, "订单 id 非法: %q", rawID)
	}
	ctx, span := g.Tracer.Start(ctx, "logic.GetOrder")
	defer span.End()
	if g.Svc == nil { // fail fast（单测契约）
		return nil, envelope.Errorf(envelope.CodeBadRequest, "order logic not configured")
	}
	o, err := g.Svc.Get(ctx, id)
	if err != nil {
		return nil, err // 透传
	}
	g.L.InfoContext(ctx, "logic.GetOrder: ok", logx.Int("order_id", int(id)))
	return o, nil
}
