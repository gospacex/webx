package service

import (
	"context"
	"testing"

	"github.com/gospacex/logx"
	"github.com/gospacex/webx/envelope"
	"github.com/gospacex/webx/examples/internal/model"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakeRepo 最小假 Repo（接口注入，不用 testify）
type fakeRepo struct {
	orders   map[int64]*model.Order
	notFound bool
}

func (f *fakeRepo) Get(ctx context.Context, id int64) (*model.Order, error) {
	if f.notFound {
		return nil, envelope.Errorf(envelope.CodeNotFound, "订单 %d 不存在", id)
	}
	if o, ok := f.orders[id]; ok {
		return o, nil
	}
	return nil, envelope.Errorf(envelope.CodeNotFound, "订单 %d 不存在", id)
}

func (f *fakeRepo) Create(ctx context.Context, o *model.Order) error { return nil }
func (f *fakeRepo) List(ctx context.Context) ([]*model.Order, error) {
	return nil, nil
}

func TestServiceGetPassthrough(t *testing.T) {
	repo := &fakeRepo{orders: map[int64]*model.Order{7: {ID: 7, Status: "pending"}}}
	svc := New(repo, logx.GetGlobal(), otel.GetTracerProvider().Tracer("t"))
	got, err := svc.Get(context.Background(), 7)
	if err != nil || got.ID != 7 {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

func TestServiceArchivedOrderRejected(t *testing.T) {
	repo := &fakeRepo{orders: map[int64]*model.Order{8: {ID: 8, Status: "archived"}}}
	svc := New(repo, logx.GetGlobal(), otel.GetTracerProvider().Tracer("t"))
	_, err := svc.Get(context.Background(), 8)
	if e := envelope.As(err); e.Code != CodeOrderArchived {
		t.Fatalf("code = %d, want %d", e.Code, CodeOrderArchived)
	}
}

func TestServiceOpensChildSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	tracer := provider.Tracer("svc-test")
	repo := &fakeRepo{orders: map[int64]*model.Order{9: {ID: 9, Status: "pending"}}}
	svc := New(repo, logx.GetGlobal(), tracer)

	// 根 span：Start 返回 (ctx 含活动 span, span)（v1.29.0 Go SDK）
	rootCtx, root := tracer.Start(context.Background(), "root")
	if _, err := svc.Get(rootCtx, 9); err != nil {
		t.Fatalf("Get: %v", err)
	}
	root.End()

	var found bool
	for _, s := range exp.GetSpans() {
		if s.Name == "service.GetOrder" && s.Parent.SpanID() == root.SpanContext().SpanID() {
			found = true
		}
	}
	if !found {
		t.Fatalf("child span 'service.GetOrder' not exported under root; got %v", exp.GetSpans())
	}
}
