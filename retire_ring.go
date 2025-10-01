package main

import "sync/atomic"

// SPSC ring for retired orders (matcher→reclaimer).
type retireRing struct {
	buf  []*Order
	mask uint64
	head uint64 // write (matcher)
	tail uint64 // read (reclaimer)
}

func newRetireRing(pow2 uint64) *retireRing {
	return &retireRing{buf: make([]*Order, pow2), mask: pow2 - 1}
}

func (q *retireRing) Enqueue(o *Order) bool {
	h := q.head
	t := atomic.LoadUint64(&q.tail)
	if h-t == uint64(len(q.buf)) {
		return false // full
	}
	q.buf[h&q.mask] = o
	q.head = h + 1
	return true
}

func (q *retireRing) Dequeue() *Order {
	t := q.tail
	h := atomic.LoadUint64(&q.head)
	if t == h {
		return nil
	}
	o := q.buf[t&q.mask]
	q.buf[t&q.mask] = nil
	q.tail = t + 1
	return o
}
