package main

type PriceLevel struct {
	Price int64
	head  *Order
	tail  *Order
}

func (lvl *PriceLevel) Enqueue(o *Order) {
	if lvl.tail != nil {
		lvl.tail.next = o
		o.prev = lvl.tail
	} else {
		lvl.head = o
	}
	lvl.tail = o
}

func (lvl *PriceLevel) unlinkAlreadyInactive(o *Order) {
	if o.prev != nil {
		o.prev.next = o.next
	} else {
		lvl.head = o.next
	}
	if o.next != nil {
		o.next.prev = o.prev
	} else {
		lvl.tail = o.prev
	}
	o.next, o.prev = nil, nil
}
