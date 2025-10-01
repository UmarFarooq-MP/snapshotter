package main

import "testing"

func TestPriceLevelFIFO(t *testing.T) {
	lvl := &PriceLevel{Price: 100}
	o1 := &Order{ID: 1}
	o2 := &Order{ID: 2}

	lvl.Enqueue(o1)
	lvl.Enqueue(o2)

	if lvl.head != o1 || lvl.tail != o2 {
		t.Error("FIFO order not maintained")
	}

	o1.Status = Inactive
	lvl.unlinkAlreadyInactive(o1)
	if lvl.head != o2 {
		t.Error("expected o2 to become head after unlinking o1")
	}
}
