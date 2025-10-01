package main

import "sync/atomic"

type OrderBook struct {
	Bids    *RBTree
	Asks    *RBTree
	LastSeq atomic.Uint64
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids: NewRBTree(),
		Asks: NewRBTree(),
	}
}

// ---------------- Matching Engine ---------------- //

// Place an order (runs matching first, then rests if needed)
func (b *OrderBook) placeOrder(
	side Side, otype OrderType, price int64,
	id uint64, qty int64, seq uint64,
	pool *OrderPool, rq *retireRing,
) *Order {
	o := pool.Get()
	if o == nil {
		panic("order pool exhausted")
	}
	*o = Order{
		ID: id, Side: side, Type: otype, Price: price,
		Qty: qty, SeqID: seq, Status: Active,
	}
	b.LastSeq.Store(seq)

	// Market orders don’t use price
	if o.Type == Market {
		o.Price = 0
	}

	// --- Special handling for FOK (dry-run) ---
	if o.Type == FOK {
		available := b.checkLiquidity(side, o.Price, o.Qty)
		if available < o.Qty {
			// Not enough liquidity → reject w/o partial fill
			o.Status = Inactive
			_ = rq.Enqueue(o)
			return o
		}
	}

	// Match against opposite side
	matched := b.match(o, rq)
	o.Filled = matched

	// Decide what to do with leftover
	switch o.Type {
	case Limit:
		if o.Qty > 0 {
			b.enqueue(o)
		}
	case PostOnly:
		if matched > 0 {
			// Rejected if it crosses
			o.Status = Inactive
			_ = rq.Enqueue(o)
		} else if o.Qty > 0 {
			b.enqueue(o)
		}
	case IOC:
		if o.Qty > 0 {
			o.Status = Inactive
			_ = rq.Enqueue(o)
		}
	case FOK:
		// At this point, full fill is guaranteed (by precheck)
		if o.Qty > 0 {
			o.Status = Inactive
			_ = rq.Enqueue(o)
		}
	case Market:
		if o.Qty > 0 {
			o.Status = Inactive
			_ = rq.Enqueue(o)
		}
	}
	return o
}

// match executes trades against opposite side
func (b *OrderBook) match(o *Order, rq *retireRing) int64 {
	filled := int64(0)

	if o.Side == Bid {
		for o.Qty > 0 {
			bestAsk := b.Asks.MinLevel()
			if bestAsk == nil || (o.Type != Market && bestAsk.Price > o.Price) {
				break
			}
			head := bestAsk.head
			trade := min(o.Qty, head.Qty)
			o.Qty -= trade
			head.Qty -= trade
			filled += trade

			if head.Qty == 0 {
				b.cancelOrder(bestAsk.Price, head, rq, Ask)
			}
		}
	} else { // Ask
		for o.Qty > 0 {
			bestBid := b.Bids.MaxLevel()
			if bestBid == nil || (o.Type != Market && bestBid.Price < o.Price) {
				break
			}
			head := bestBid.head
			trade := min(o.Qty, head.Qty)
			o.Qty -= trade
			head.Qty -= trade
			filled += trade

			if head.Qty == 0 {
				b.cancelOrder(bestBid.Price, head, rq, Bid)
			}
		}
	}
	return filled
}

// enqueue leftover order into book
func (b *OrderBook) enqueue(o *Order) {
	if o.Side == Bid {
		lvl := b.Bids.UpsertLevel(o.Price)
		lvl.Enqueue(o)
	} else {
		lvl := b.Asks.UpsertLevel(o.Price)
		lvl.Enqueue(o)
	}
}

// cancel order and recycle
func (b *OrderBook) cancelOrder(price int64, o *Order, rq *retireRing, side Side) {
	o.Status = Inactive
	o.retireEpoch = globalEpoch.Load()

	var lvl *PriceLevel
	if side == Bid {
		lvl = b.Bids.FindLevel(price)
	} else {
		lvl = b.Asks.FindLevel(price)
	}

	if lvl != nil {
		lvl.unlinkAlreadyInactive(o)
		if lvl.head == nil {
			if side == Bid {
				_ = b.Bids.DeleteLevel(price)
			} else {
				_ = b.Asks.DeleteLevel(price)
			}
		}
	}
	if !rq.Enqueue(o) {
		panic("retire ring full")
	}
}

// ---------------- FOK Pre-check ---------------- //

// checkLiquidity returns total qty available on the opposite side up to price limit
func (b *OrderBook) checkLiquidity(side Side, limitPrice int64, desired int64) int64 {
	available := int64(0)
	if side == Bid {
		b.Asks.ForEachAscending(func(lvl *PriceLevel) bool {
			if lvl.Price > limitPrice {
				return false
			}
			available += lvl.TotalQty
			if available >= desired {
				return false
			}
			return true
		})
	} else {
		b.Bids.ForEachDescending(func(lvl *PriceLevel) bool {
			if lvl.Price < limitPrice {
				return false
			}
			available += lvl.TotalQty
			if available >= desired {
				return false
			}
			return true
		})
	}
	return available
}

// ---------------- Epoch Reclaim ---------------- //

func advanceEpochAndReclaim(rq *retireRing, pool *OrderPool, rs ...*Reader) {
	globalEpoch.Add(1)
	min := minReaderEpoch(rs...)
	for {
		o := rq.Dequeue()
		if o == nil {
			break
		}
		if min == ^uint64(0) || o.retireEpoch < min {
			pool.Put(o)
		} else {
			_ = rq.Enqueue(o)
			break
		}
	}
}

// ---------------- Snapshots ---------------- //

func (b *OrderBook) SnapshotActiveIter(r *Reader, visit func(price int64, o *Order)) {
	r.EnterRead()
	// Bids descending (highest first)
	b.Bids.ForEachDescending(func(lvl *PriceLevel) bool {
		for n := lvl.head; n != nil; n = n.next {
			if n.Status == Active {
				visit(lvl.Price, n)
			}
		}
		return true
	})
	// Asks ascending (lowest first)
	b.Asks.ForEachAscending(func(lvl *PriceLevel) bool {
		for n := lvl.head; n != nil; n = n.next {
			if n.Status == Active {
				visit(lvl.Price, n)
			}
		}
		return true
	})
	r.ExitRead()
}

// helper
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
