// model 订单实体 + 数据访问（内存 demo 实现）
package model

import (
	"context"
	"sync"

	"github.com/gospacex/webx/envelope"
)

// Order 订单实体
type Order struct {
	ID     int64
	Item   string
	Qty    int
	Status string
}

// OrderRepository 数据访问接口（业务层依赖抽象）
type OrderRepository interface {
	Get(ctx context.Context, id int64) (*Order, error)
	Create(ctx context.Context, o *Order) error
	List(ctx context.Context) ([]*Order, error)
}

// MemRepo 内存实现（demo）
type MemRepo struct {
	mu sync.Mutex
	m  map[int64]*Order
}

// NewMemRepo 创建空内存 Repo
func NewMemRepo() *MemRepo {
	return &MemRepo{m: make(map[int64]*Order)}
}

// Get 不存在 → envelope 1004
func (r *MemRepo) Get(ctx context.Context, id int64) (*Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o, ok := r.m[id]; ok {
		return o, nil
	}
	return nil, envelope.Errorf(envelope.CodeNotFound, "订单 %d 不存在", id)
}

// Create 入库（demo 语义：ID 必填，重复 1009）
func (r *MemRepo) Create(ctx context.Context, o *Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.m[o.ID]; exists {
		return envelope.Errorf(envelope.CodeConflict, "订单 %d 已存在", o.ID)
	}
	r.m[o.ID] = o
	return nil
}

// List 全量（demo 不分页）
func (r *MemRepo) List(ctx context.Context) ([]*Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Order, 0, len(r.m))
	for _, o := range r.m {
		out = append(out, o)
	}
	return out, nil
}
