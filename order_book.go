package main

import "sync/atomic"

type OrderBook struct {
	Tree    *RBTree
	LastSeq atomic.Uint64
}

func NewOrderBook() *OrderBook {
	return &OrderBook{Tree: NewRBTree()}
}

// Place/append order (hot path; no allocations for orders)
func (b *OrderBook) placeOrder(price int64, id uint64, qty int64, seq uint64, pool *OrderPool) *Order {
	o := pool.Get()
	if o == nil {
		panic("order pool exhausted")
	}
	o.ID, o.Qty, o.SeqID, o.Status = id, qty, seq, Active

	lvl := b.Tree.FindLevel(price)
	if lvl == nil {
		lvl = b.Tree.UpsertLevel(price)
	}
	lvl.Enqueue(o)
	b.LastSeq.Store(seq)
	return o
}

// Cancel/retire
func (b *OrderBook) cancelOrder(price int64, o *Order, rq *retireRing) {
	o.Status = Inactive
	o.retireEpoch = globalEpoch.Load()

	lvl := b.Tree.FindLevel(price)
	if lvl != nil {
		lvl.unlinkAlreadyInactive(o)
		if lvl.head == nil {
			_ = b.Tree.DeleteLevel(price) // prune empty level
		}
	}
	if !rq.Enqueue(o) {
		panic("retire ring full")
	}
}

// Epoch maintenance
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

// Snapshot
func (b *OrderBook) SnapshotActiveIter(r *Reader, visit func(price int64, o *Order)) {
	r.EnterRead()
	b.Tree.ForEachAscending(func(lvl *PriceLevel) bool {
		for n := lvl.head; n != nil; n = n.next {
			if n.Status == Active {
				visit(lvl.Price, n)
			}
		}
		return true
	})
	r.ExitRead()
}
