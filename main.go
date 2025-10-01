package main

import (
	"fmt"
	"runtime"
)

var (
	// Bigger pools/rings for 200k+ TPS
	orderPool = NewOrderPool(1 << 20)  // 1M orders
	retireQ   = newRetireRing(1 << 18) // 256k retired
	reader    Reader
	book      = NewOrderBook()
)

func main() {
	// Pin matcher to dedicated OS thread (avoid scheduler migration)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	globalEpoch.Store(100)

	// --- Demo: Add initial orders --- //
	fmt.Println("Placing initial bid/ask orders...")

	// Place a bid @100
	o1 := book.placeOrder(Bid, Limit, 100, 1, 10_000, 1, orderPool, retireQ)
	// Place another bid @100
	_ = book.placeOrder(Bid, Limit, 100, 2, 20_000, 2, orderPool, retireQ)
	// Place an ask @101
	_ = book.placeOrder(Ask, Limit, 101, 3, 15_000, 3, orderPool, retireQ)

	fmt.Println("Init snapshot:")
	book.SnapshotActiveIter(&reader, func(p int64, o *Order) {
		side := "BID"
		if o.Side == Ask {
			side = "ASK"
		}
		fmt.Printf("  %s %d: O%d qty=%d\n", side, p, o.ID, o.Qty)
	})

	// --- Cancel demo --- //
	book.cancelOrder(o1.Price, o1, retireQ, o1.Side)

	// Snapshot in parallel
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread() // pin snapshotter too
		defer runtime.UnlockOSThread()
		book.SnapshotActiveIter(&reader, func(p int64, o *Order) {
			side := "BID"
			if o.Side == Ask {
				side = "ASK"
			}
			fmt.Printf("[snap] %s %d: O%d\n", side, p, o.ID)
		})
		close(done)
	}()

	// Place IOC order (buy that should cancel leftover)
	_ = book.placeOrder(Bid, IOC, 101, 4, 5_000, 4, orderPool, retireQ)

	// First reclaim (reader active → canceled not yet recycled)
	advanceEpochAndReclaim(retireQ, orderPool, &reader)

	<-done

	// Second reclaim (reader done → canceled recycled)
	advanceEpochAndReclaim(retireQ, orderPool, &reader)

	// --- Final snapshot --- //
	fmt.Println("Final snapshot:")
	book.SnapshotActiveIter(&reader, func(p int64, o *Order) {
		side := "BID"
		if o.Side == Ask {
			side = "ASK"
		}
		fmt.Printf("  %s %d: O%d qty=%d\n", side, p, o.ID, o.Qty)
	})
}
