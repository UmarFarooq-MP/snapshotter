package main

import "testing"

func TestOrderBookBasicFlow(t *testing.T) {
	book := NewOrderBook()
	pool := NewOrderPool(8)
	rq := newRetireRing(8)

	// Place first order at price=100
	o1 := book.placeOrder(100, 1, 1000, 1, pool)
	if o1 == nil || o1.Status != Active || o1.Qty != 1000 {
		t.Fatalf("order not placed correctly: %+v", o1)
	}
	lvl := book.Tree.FindLevel(100)
	if lvl == nil {
		t.Fatal("expected price level 100 to exist")
	}
	// With a single order, head and tail should both be o1
	if lvl.head != o1 || lvl.tail != o1 {
		t.Errorf("expected FIFO with single order; got head=%v tail=%v", lvl.head, lvl.tail)
	}

	// Cancel o1 → should unlink from the level and enqueue to retire ring
	book.cancelOrder(100, o1, rq)
	if o1.Status != Inactive {
		t.Error("order should be inactive after cancel")
	}
	if lvl.head != nil || lvl.tail != nil {
		t.Error("expected empty level after unlinking the only order")
	}

	// Advance epoch and reclaim with no active readers → o1 returns to pool
	advanceEpochAndReclaim(rq, pool, &Reader{})
	// (Optional) sanity: pool should not be empty now
	if got := pool.Get(); got == nil {
		t.Error("expected reclaimed order in pool")
	} else {
		// put it back so pool capacity is stable for the rest of the test
		pool.Put(got)
	}

	// Place two more orders at the same price (FIFO check)
	o2 := book.placeOrder(100, 2, 2000, 2, pool)
	o3 := book.placeOrder(100, 3, 3000, 3, pool)
	lvl = book.Tree.FindLevel(100)
	if lvl == nil {
		t.Fatal("expected price level 100 to exist after new placements")
	}
	if lvl.head != o2 || lvl.tail != o3 {
		t.Errorf("expected head=o2, tail=o3; got head=%v tail=%v", lvl.head, lvl.tail)
	}

	// Snapshot should visit only ACTIVE orders in price order
	var seenIDs []uint64
	book.SnapshotActiveIter(&Reader{}, func(price int64, o *Order) {
		if price != 100 {
			t.Errorf("unexpected price in snapshot: %d", price)
		}
		seenIDs = append(seenIDs, o.ID)
	})
	if len(seenIDs) != 2 || seenIDs[0] != 2 || seenIDs[1] != 3 {
		t.Errorf("snapshot mismatch; got %v, want [2 3]", seenIDs)
	}
}
