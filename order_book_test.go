package main

import "testing"

func newTestEnv() (*OrderBook, *OrderPool, *retireRing) {
	return NewOrderBook(), NewOrderPool(1 << 12), newRetireRing(1 << 12)
}

func TestLimitOrderInsertAndMatch(t *testing.T) {
	book, pool, rq := newTestEnv()

	// Place a bid @100
	bid := book.placeOrder(Bid, Limit, 100, 1, 10, 1, pool, rq)
	if bid == nil || bid.Status != Active {
		t.Fatal("expected active bid order")
	}

	// Place an ask @100 (crosses immediately)
	ask := book.placeOrder(Ask, Limit, 100, 2, 10, 2, pool, rq)

	if bid.Qty != 0 || ask.Qty != 0 {
		t.Errorf("expected both fully filled, got bidQty=%d askQty=%d", bid.Qty, ask.Qty)
	}
}

func TestIOCOrder(t *testing.T) {
	book, pool, rq := newTestEnv()

	// Add some resting ask @100
	_ = book.placeOrder(Ask, Limit, 100, 1, 5, 1, pool, rq)

	// Place IOC bid @100 qty=10 (only 5 available)
	bid := book.placeOrder(Bid, IOC, 100, 2, 10, 2, pool, rq)

	if bid.Status != Inactive {
		t.Error("IOC should be inactive after placement")
	}
	if bid.Qty != 5 { // 10 wanted, 5 filled, 5 canceled
		t.Errorf("expected leftover canceled=5, got %d", bid.Qty)
	}
}

func TestFOKOrder(t *testing.T) {
	book, pool, rq := newTestEnv()

	// Only 5 ask liquidity available
	_ = book.placeOrder(Ask, Limit, 100, 1, 5, 1, pool, rq)

	// Place FOK bid @100 qty=10 (not enough liquidity)
	bid := book.placeOrder(Bid, FOK, 100, 2, 10, 2, pool, rq)

	if bid.Status != Inactive {
		t.Error("FOK should be canceled when not fully matched")
	}
	if bid.Filled != 0 {
		t.Errorf("FOK should have no partial fill, got %d", bid.Filled)
	}
}

func TestPostOnlyOrder(t *testing.T) {
	book, pool, rq := newTestEnv()

	// Add ask @100
	_ = book.placeOrder(Ask, Limit, 100, 1, 5, 1, pool, rq)

	// Place PostOnly bid @101 (would cross)
	bid := book.placeOrder(Bid, PostOnly, 101, 2, 5, 2, pool, rq)

	if bid.Status != Inactive {
		t.Error("PostOnly should be rejected if it crosses")
	}

	// Place PostOnly bid @99 (does not cross, should rest)
	bid2 := book.placeOrder(Bid, PostOnly, 99, 3, 5, 3, pool, rq)
	if bid2.Status != Active {
		t.Error("PostOnly should rest if it does not cross")
	}
}

func TestBidAskSeparation(t *testing.T) {
	book, pool, rq := newTestEnv()

	// Place bid @99
	_ = book.placeOrder(Bid, Limit, 99, 1, 5, 1, pool, rq)
	// Place ask @101
	_ = book.placeOrder(Ask, Limit, 101, 2, 5, 2, pool, rq)

	bestBid := book.Bids.MaxLevel()
	bestAsk := book.Asks.MinLevel()

	if bestBid == nil || bestAsk == nil {
		t.Fatal("expected both sides populated")
	}
	if bestBid.Price >= bestAsk.Price {
		t.Errorf("expected bestBid < bestAsk, got %d >= %d", bestBid.Price, bestAsk.Price)
	}
}

func TestCancelAndReclaim(t *testing.T) {
	book, pool, rq := newTestEnv()

	o1 := book.placeOrder(Bid, Limit, 100, 1, 5, 1, pool, rq)
	book.cancelOrder(100, o1, rq, Bid)
	if o1.Status != Inactive {
		t.Error("expected inactive after cancel")
	}

	advanceEpochAndReclaim(rq, pool, &Reader{})
	if got := pool.Get(); got == nil {
		t.Error("expected order recycled into pool")
	} else {
		pool.Put(got) // return
	}
}

func TestSnapshotIter(t *testing.T) {
	book, pool, rq := newTestEnv()
	_ = book.placeOrder(Bid, Limit, 99, 1, 5, 1, pool, rq)
	_ = book.placeOrder(Ask, Limit, 101, 2, 5, 2, pool, rq)

	r := &Reader{}
	var seen []uint64
	book.SnapshotActiveIter(r, func(p int64, o *Order) {
		seen = append(seen, o.ID)
	})

	if len(seen) != 2 {
		t.Errorf("expected 2 orders in snapshot, got %d", len(seen))
	}
}
