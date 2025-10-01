package main

import "testing"

func TestOrderPoolLifecycle(t *testing.T) {
	pool := NewOrderPool(2)

	o1 := pool.Get()
	if o1 == nil || o1.Status != Active {
		t.Fatal("expected active order from pool")
	}
	pool.Put(o1)
	if o1.Status != Inactive {
		t.Error("expected inactive after Put")
	}

	o2 := pool.Get()
	o3 := pool.Get()
	if o2 == nil || o3 == nil {
		t.Error("expected to get preallocated orders")
	}
}
