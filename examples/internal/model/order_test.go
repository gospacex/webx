package model

import (
	"context"
	"errors"
	"testing"

	"github.com/gospacex/webx/envelope"
)

func TestMemRepoCRUD(t *testing.T) {
	repo := NewMemRepo()
	ctx := context.Background()
	want := &Order{ID: 1, Item: "pen", Qty: 2, Status: "pending"}
	if err := repo.Create(ctx, want); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Item != "pen" || got.Qty != 2 {
		t.Fatalf("Get = %+v, want %+v", got, want)
	}
	if list, err := repo.List(ctx); err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v; want 1 item", list, err)
	}
}

func TestMemRepoGetMissingReturns404(t *testing.T) {
	repo := NewMemRepo()
	_, err := repo.Get(context.Background(), 99)
	if e := envelope.As(err); e.Code != envelope.CodeNotFound {
		t.Fatalf("Get missing code = %d, want %d", e.Code, envelope.CodeNotFound)
	}
	if !errors.Is(err, envelope.ErrNotFound) {
		t.Fatal("errors.Is(Get missing, ErrNotFound) = false")
	}
}
