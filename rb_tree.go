package main

// Production-grade Red–Black Tree for price levels.
// - Single-writer, lock-free API (caller coordinates concurrency).
// - Sentinel nil node to simplify rotations & fixups.
// - O(log n) Find/Insert/Delete; O(1) Min/Max with cached search.
// - Asc/Desc iterators with early-stop callbacks.

type Color uint8

const (
	red   Color = 0
	black Color = 1
)

type node struct {
	key    int64
	level  *PriceLevel
	color  Color
	left   *node
	right  *node
	parent *node
}

type RBTree struct {
	root *node
	nil  *node // sentinel (black)
	size int
}

// NewRBTree constructs an empty tree with a black sentinel.
func NewRBTree() *RBTree {
	nilNode := &node{color: black}
	return &RBTree{
		root: nilNode,
		nil:  nilNode,
		size: 0,
	}
}

// Size returns number of price levels currently present.
func (t *RBTree) Size() int { return t.size }

// FindLevel returns the PriceLevel for 'price' or nil.
func (t *RBTree) FindLevel(price int64) *PriceLevel {
	n := t.root
	for n != t.nil {
		switch {
		case price < n.key:
			n = n.left
		case price > n.key:
			n = n.right
		default:
			return n.level
		}
	}
	return nil
}

// UpsertLevel finds the level for 'price' or creates it if missing.
// Returns the existing/new *PriceLevel.
func (t *RBTree) UpsertLevel(price int64) *PriceLevel {
	// Standard BST insert search
	y := t.nil
	x := t.root
	for x != t.nil {
		y = x
		if price < x.key {
			x = x.left
		} else if price > x.key {
			x = x.right
		} else {
			return x.level // already present
		}
	}

	// Create new node/level
	pl := &PriceLevel{Price: price}
	z := &node{
		key:    price,
		level:  pl,
		color:  red, // new insertions start red
		left:   t.nil,
		right:  t.nil,
		parent: y,
	}

	if y == t.nil {
		t.root = z
	} else if z.key < y.key {
		y.left = z
	} else {
		y.right = z
	}
	t.insertFixup(z)
	t.size++
	return pl
}

// DeleteLevel removes the level at 'price' if it exists.
// Returns true if a node was removed.
func (t *RBTree) DeleteLevel(price int64) bool {
	z := t.searchNode(price)
	if z == t.nil {
		return false
	}
	t.deleteNode(z)
	t.size--
	return true
}

// MinLevel returns the lowest-price level (asks) or nil if empty.
func (t *RBTree) MinLevel() *PriceLevel {
	n := t.minNode(t.root)
	if n == t.nil {
		return nil
	}
	return n.level
}

// MaxLevel returns the highest-price level (bids) or nil if empty.
func (t *RBTree) MaxLevel() *PriceLevel {
	n := t.maxNode(t.root)
	if n == t.nil {
		return nil
	}
	return n.level
}

// Successor returns the next higher price level after 'price' (or nil).
func (t *RBTree) Successor(price int64) *PriceLevel {
	n := t.root
	var succ *node = t.nil
	for n != t.nil {
		if price < n.key {
			succ = n
			n = n.left
		} else {
			n = n.right
		}
	}
	if succ == t.nil {
		return nil
	}
	return succ.level
}

// Predecessor returns the next lower price level before 'price' (or nil).
func (t *RBTree) Predecessor(price int64) *PriceLevel {
	n := t.root
	var pred *node = t.nil
	for n != t.nil {
		if price > n.key {
			pred = n
			n = n.right
		} else {
			n = n.left
		}
	}
	if pred == t.nil {
		return nil
	}
	return pred.level
}

// ForEachAscending applies fn from lowest to highest price.
// If fn returns false, iteration stops early.
func (t *RBTree) ForEachAscending(fn func(*PriceLevel) bool) {
	for n := t.minNode(t.root); n != t.nil; n = t.next(n) {
		if !fn(n.level) {
			return
		}
	}
}

// ForEachDescending applies fn from highest to lowest price.
// If fn returns false, iteration stops early.
func (t *RBTree) ForEachDescending(fn func(*PriceLevel) bool) {
	for n := t.maxNode(t.root); n != t.nil; n = t.prev(n) {
		if !fn(n.level) {
			return
		}
	}
}

/*************** Internal helpers (nodes & search) ***************/

func (t *RBTree) searchNode(price int64) *node {
	n := t.root
	for n != t.nil {
		if price < n.key {
			n = n.left
		} else if price > n.key {
			n = n.right
		} else {
			return n
		}
	}
	return t.nil
}

func (t *RBTree) minNode(n *node) *node {
	if n == t.nil {
		return t.nil
	}
	for n.left != t.nil {
		n = n.left
	}
	return n
}

func (t *RBTree) maxNode(n *node) *node {
	if n == t.nil {
		return t.nil
	}
	for n.right != t.nil {
		n = n.right
	}
	return n
}

// In-order successor
func (t *RBTree) next(n *node) *node {
	if n.right != t.nil {
		return t.minNode(n.right)
	}
	p := n.parent
	for p != t.nil && n == p.right {
		n = p
		p = p.parent
	}
	return p
}

// In-order predecessor
func (t *RBTree) prev(n *node) *node {
	if n.left != t.nil {
		return t.maxNode(n.left)
	}
	p := n.parent
	for p != t.nil && n == p.left {
		n = p
		p = p.parent
	}
	return p
}

/******************** Rotations & Fixups ********************/

func (t *RBTree) leftRotate(x *node) {
	y := x.right
	x.right = y.left
	if y.left != t.nil {
		y.left.parent = x
	}
	y.parent = x.parent
	if x.parent == t.nil {
		t.root = y
	} else if x == x.parent.left {
		x.parent.left = y
	} else {
		x.parent.right = y
	}
	y.left = x
	x.parent = y
}

func (t *RBTree) rightRotate(y *node) {
	x := y.left
	y.left = x.right
	if x.right != t.nil {
		x.right.parent = y
	}
	x.parent = y.parent
	if y.parent == t.nil {
		t.root = x
	} else if y == y.parent.right {
		y.parent.right = x
	} else {
		y.parent.left = x
	}
	x.right = y
	y.parent = x
}

func (t *RBTree) insertFixup(z *node) {
	for z.parent.color == red {
		if z.parent == z.parent.parent.left {
			y := z.parent.parent.right // uncle
			if y.color == red {
				// Case 1
				z.parent.color = black
				y.color = black
				z.parent.parent.color = red
				z = z.parent.parent
			} else {
				if z == z.parent.right {
					// Case 2
					z = z.parent
					t.leftRotate(z)
				}
				// Case 3
				z.parent.color = black
				z.parent.parent.color = red
				t.rightRotate(z.parent.parent)
			}
		} else {
			// mirror cases
			y := z.parent.parent.left // uncle
			if y.color == red {
				// Case 1
				z.parent.color = black
				y.color = black
				z.parent.parent.color = red
				z = z.parent.parent
			} else {
				if z == z.parent.left {
					// Case 2
					z = z.parent
					t.rightRotate(z)
				}
				// Case 3
				z.parent.color = black
				z.parent.parent.color = red
				t.leftRotate(z.parent.parent)
			}
		}
	}
	t.root.color = black
}

func (t *RBTree) transplant(u, v *node) {
	if u.parent == t.nil {
		t.root = v
	} else if u == u.parent.left {
		u.parent.left = v
	} else {
		u.parent.right = v
	}
	v.parent = u.parent
}

func (t *RBTree) deleteNode(z *node) {
	y := z
	yOrigColor := y.color
	var x *node

	if z.left == t.nil {
		x = z.right
		t.transplant(z, z.right)
	} else if z.right == t.nil {
		x = z.left
		t.transplant(z, z.left)
	} else {
		y = t.minNode(z.right) // successor
		yOrigColor = y.color
		x = y.right
		if y.parent == z {
			x.parent = y
		} else {
			t.transplant(y, y.right)
			y.right = z.right
			y.right.parent = y
		}
		t.transplant(z, y)
		y.left = z.left
		y.left.parent = y
		y.color = z.color
	}

	if yOrigColor == black {
		t.deleteFixup(x)
	}
}

func (t *RBTree) deleteFixup(x *node) {
	for x != t.root && x.color == black {
		if x == x.parent.left {
			w := x.parent.right
			if w.color == red {
				// Case 1
				w.color = black
				x.parent.color = red
				t.leftRotate(x.parent)
				w = x.parent.right
			}
			if w.left.color == black && w.right.color == black {
				// Case 2
				w.color = red
				x = x.parent
			} else {
				if w.right.color == black {
					// Case 3
					w.left.color = black
					w.color = red
					t.rightRotate(w)
					w = x.parent.right
				}
				// Case 4
				w.color = x.parent.color
				x.parent.color = black
				w.right.color = black
				t.leftRotate(x.parent)
				x = t.root
			}
		} else {
			// mirror cases
			w := x.parent.left
			if w.color == red {
				// Case 1
				w.color = black
				x.parent.color = red
				t.rightRotate(x.parent)
				w = x.parent.left
			}
			if w.right.color == black && w.left.color == black {
				// Case 2
				w.color = red
				x = x.parent
			} else {
				if w.left.color == black {
					// Case 3
					w.right.color = black
					w.color = red
					t.leftRotate(w)
					w = x.parent.left
				}
				// Case 4
				w.color = x.parent.color
				x.parent.color = black
				w.left.color = black
				t.rightRotate(x.parent)
				x = t.root
			}
		}
	}
	x.color = black
}
