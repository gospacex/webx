// service 业务核心：错误在此层产生/转换，子 span 入链
package service

import (
	"context"

	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/model"
	"go.opentelemetry.io/otel/trace"
)

// OrderService 订单业务核心
type OrderService struct {
	Repo   model.OrderRepository
	L      logx.Logger
	Tracer trace.Tracer
}

// New 构造（依赖注入）
func New(repo model.OrderRepository, l logx.Logger, tr trace.Tracer) *OrderService {
	return &OrderService{Repo: repo, L: l, Tracer: tr}
}

// Get 取订单；model 错误按码透传；归档订单 → 2001
func (s *OrderService) Get(ctx context.Context, id int64) (*model.Order, error) {
	ctx, span := s.Tracer.Start(ctx, "service.GetOrder")
	defer span.End()
	o, err := s.Repo.Get(ctx, id)
	if err != nil {
		s.L.ErrorContext(ctx, "service.GetOrder: repo error",
			logx.Int("order_id", int(id)), logx.Err(err))
		return nil, err // 透传（model 已给 envelope 码）
	}
	if o.Status == "archived" {
		err := envelope.Errorf(CodeOrderArchived, "订单 %d 已归档", id)
		s.L.WarnContext(ctx, "service.GetOrder: order archived", logx.Int("order_id", int(id)))
		return nil, err
	}
	s.L.InfoContext(ctx, "service.GetOrder: ok", logx.Int("order_id", int(id)))
	return o, nil
}
