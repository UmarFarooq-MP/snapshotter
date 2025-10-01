package main

type OrderStatus uint8

const (
	Active OrderStatus = iota
	Inactive
)

type Order struct {
	ID          uint64
	Qty         int64
	SeqID       uint64
	Status      OrderStatus
	next, prev  *Order   // FIFO links (matcher owns them)
	retireEpoch uint64   // epoch when retired
	_           [32]byte // padding (avoid false sharing if batching stats)
}

// OrderPool Fixed-capacity stack pool (no append; no GC in steady state)
type OrderPool struct {
	store []*Order
	top   int
}

func NewOrderPool(cap int) *OrderPool {
	p := &OrderPool{store: make([]*Order, cap), top: cap}
	for i := 0; i < cap; i++ {
		p.store[i] = new(Order) // allocate once at startup
	}
	return p
}

func (p *OrderPool) Get() *Order {
	if p.top == 0 {
		return nil // exhausted
	}
	p.top--
	o := p.store[p.top]
	*o = Order{Status: Active} // reset
	return o
}

func (p *OrderPool) Put(o *Order) {
	if p.top == len(p.store) {
		return // pool full
	}
	o.next, o.prev = nil, nil
	o.Status = Inactive
	p.store[p.top] = o
	p.top++
}
