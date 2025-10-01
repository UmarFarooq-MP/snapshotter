package main

import "fmt"

var (
	orderPool = NewOrderPool(1 << 16)  // 65,536 orders
	retireQ   = newRetireRing(1 << 15) // 32,768 retired
	reader    Reader
	book      = NewOrderBook()
)

func main() {
	globalEpoch.Store(100)

	// Insert 2 orders at price 100
	o1 := book.placeOrder(100, 1, 10_000, 1, orderPool)
	_ = book.placeOrder(100, 2, 20_000, 2, orderPool)

	fmt.Println("Init snapshot:")
	book.SnapshotActiveIter(&reader, func(p int64, o *Order) {
		fmt.Printf("  %d: O%d qty=%d\n", p, o.ID, o.Qty)
	})

	// Cancel o1
	book.cancelOrder(100, o1, retireQ)

	// Snapshot in parallel
	done := make(chan struct{})
	go func() {
		book.SnapshotActiveIter(&reader, func(p int64, o *Order) {
			fmt.Printf("[snap] %d: O%d\n", p, o.ID)
		})
		close(done)
	}()

	// Place O3
	_ = book.placeOrder(100, 3, 5_000, 3, orderPool)

	// First reclaim (reader active → O1 not recycled)
	advanceEpochAndReclaim(retireQ, orderPool, &reader)

	<-done

	// Second reclaim (reader done → O1 recycled)
	advanceEpochAndReclaim(retireQ, orderPool, &reader)

	fmt.Println("Final snapshot (O2 & O3 only):")
	book.SnapshotActiveIter(&reader, func(p int64, o *Order) {
		fmt.Printf("  %d: O%d qty=%d\n", p, o.ID, o.Qty)
	})
}
