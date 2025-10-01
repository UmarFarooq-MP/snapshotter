package main

import (
	"sync/atomic"
	"testing"
)

func BenchmarkPlaceOrder(b *testing.B) {
	book := NewOrderBook()
	pool := NewOrderPool(b.N)
	seq := uint64(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = book.placeOrder(100, uint64(i), 1000, seq, pool)
		seq++
	}
}

func BenchmarkCancelOrder(b *testing.B) {
	book := NewOrderBook()
	pool := NewOrderPool(b.N)
	rq := newRetireRing(uint64(b.N) * 2)

	// pre-fill with orders
	var orders []*Order
	for i := 0; i < b.N; i++ {
		o := book.placeOrder(100, uint64(i), 1000, uint64(i+1), pool)
		orders = append(orders, o)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		book.cancelOrder(100, orders[i], rq)
	}
}

func BenchmarkSnapshot(b *testing.B) {
	book := NewOrderBook()
	pool := NewOrderPool(1 << 16) // 65k orders
	for i := 0; i < 50000; i++ {
		_ = book.placeOrder(int64(100+i%10), uint64(i), 1000, uint64(i+1), pool)
	}
	reader := &Reader{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		book.SnapshotActiveIter(reader, func(p int64, o *Order) {
			count++
		})
		if count == 0 {
			b.Fatal("snapshot returned no orders")
		}
	}
}

func BenchmarkMixedPlaceCancel(b *testing.B) {
	book := NewOrderBook()
	pool := NewOrderPool(b.N * 2)
	rq := newRetireRing(uint64(b.N) * 2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o := book.placeOrder(100, uint64(i), 1000, uint64(i+1), pool)
		if i%2 == 0 {
			book.cancelOrder(100, o, rq)
		}
	}
}

//
// 🔥 Parallel benchmarks (simulate concurrency)
//

// Readers vs Writers: many goroutines snapshot while matcher inserts.
func BenchmarkParallelPlaceAndSnapshot(b *testing.B) {
	book := NewOrderBook()
	pool := NewOrderPool(b.N * 2)
	seq := uint64(1)
	reader := &Reader{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		localSeq := atomic.AddUint64(&seq, 1)
		for pb.Next() {
			// alternate: odd goroutines place orders, even goroutines snapshot
			if localSeq%2 == 0 {
				_ = book.placeOrder(100, localSeq, 1000, localSeq, pool)
			} else {
				book.SnapshotActiveIter(reader, func(p int64, o *Order) {})
			}
			localSeq = atomic.AddUint64(&seq, 1)
		}
	})
}

// Cancels + Snapshots in parallel.
func BenchmarkParallelCancelAndSnapshot(b *testing.B) {
	book := NewOrderBook()
	pool := NewOrderPool(b.N * 2)
	rq := newRetireRing(uint64(b.N) * 2)
	reader := &Reader{}

	// pre-fill orders
	orders := make([]*Order, b.N)
	for i := 0; i < b.N; i++ {
		orders[i] = book.placeOrder(100, uint64(i), 1000, uint64(i+1), pool)
	}
	idx := int64(0)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n := atomic.AddInt64(&idx, 1)
			if n%2 == 0 {
				// cancel
				i := int(n) % len(orders)
				book.cancelOrder(100, orders[i], rq)
			} else {
				// snapshot
				book.SnapshotActiveIter(reader, func(p int64, o *Order) {})
			}
		}
	})
}
