// Package etree provides a red-black tree for events.
// This package was intended for aggregate streams but isn't currently used by
// any package. TODO: delete package (?)
package etree

import (
	"errors"
	"math"

	"github.com/modernice/goes/event"
)

const (
	black = color(true)
	red   = color(false)
)

var (
	errConsecutiveRed = errors.New("consecutive red nodes")
	errInvalidOrder   = errors.New("invalid order")
)

// Tree is a red-black tree that orders events by their aggregate version.
type Tree struct {
	root *node
	size int
}

type node struct {
	evt    event.Event
	color  color
	parent *node
	left   *node
	right  *node
}

type color bool

// Insert inserts an Event into the Tree.
func (t *Tree) Insert(evt event.Event) {
	var n *node
	if t.root != nil {
		n = t.root.insert(evt)
	} else {
		n = &node{evt: evt}
		t.root = n
	}
	t.finalizeInsert(n)
	t.size++
}

// Size returns the number of elements in the Tree.
func (t *Tree) Size() int {
	return t.size
}

// Matrix returns the Tree as a slice of levels.
func (t *Tree) Matrix() [][]event.Event {
	nrows := t.height()
	rows := make([][]event.Event, nrows)
	for i := range rows {
		rows[i] = t.rowMatrix(i)
	}
	return rows
}

func (t *Tree) height() int {
	if t.root == nil {
		return 0
	}
	return t.root.maxDepth() + 1
}

func (t *Tree) rowMatrix(i int) []event.Event {
	nodes := t.root.level(i)
	events := make([]event.Event, len(nodes))
	for i, n := range nodes {
		events[i] = n.evt
	}
	return events
}

func (t *Tree) level(l int) []*node {
	h := t.height()
	if h <= l {
		return nil
	}
	return t.root.level(l)
}

// Walk walks the Tree in-order and calls fn on every event.
func (t *Tree) Walk(fn func(event.Event)) {
	t.walk(func(n *node) { fn(n.evt) })
}

func (t *Tree) walk(fn func(*node)) {
	if t.root != nil {
		t.root.walk(fn)
	}
}

func (t *Tree) finalizeInsert(n *node) {
	// case 1
	if n.parent == nil {
		n.color = black
		return
	}

	// case 2
	if n.parent.color == black {
		return
	}

	uncle := n.uncle()
	if uncle != nil && uncle.color == red {
		t.finalizeInsertCase3(n)
		return
	}

	t.finalizeInsertCase4(n)
}

func (t *Tree) finalizeInsertCase3(n *node) {
	n.parent.color = black
	n.uncle().color = black
	gp := n.grandparent()
	gp.color = red
	t.finalizeInsert(gp)
}

func (t *Tree) finalizeInsertCase4(n *node) {
	p := n.parent
	gp := n.grandparent()

	if isRightOf(p, n) && isLeftOf(gp, p) {
		t.rotateLeft(p)
		n = n.left
	} else if isLeftOf(p, n) && isRightOf(gp, p) {
		t.rotateRight(p)
		n = n.right
	}

	p = n.parent
	gp = n.grandparent()

	if isLeftOf(p, n) {
		t.rotateRight(gp)
	} else {
		t.rotateLeft(gp)
	}

	p.color = black
	gp.color = red
}

func (t *Tree) rotateLeft(n *node) {
	r := n.right
	n.right = r.left
	if n.right != nil {
		n.right.parent = n
	}
	r.parent = n.parent
	if n.parent == nil {
		t.root = r
	} else if n == n.parent.left {
		n.parent.left = r
	} else {
		n.parent.right = r
	}
	r.left = n
	n.parent = r
}

func (t *Tree) rotateRight(n *node) {
	l := n.left
	n.left = l.right
	if n.left != nil {
		n.left.parent = n
	}
	l.parent = n.parent
	if n.parent == nil {
		t.root = l
	} else if n == n.parent.left {
		n.parent.left = l
	} else {
		n.parent.right = l
	}
	l.right = n
	n.parent = l
}

func (t *Tree) validate() error {
	return t.root.validate()
}

func (n *node) validate() error {
	if n == nil {
		return nil
	}

	if n.parent != nil {
		if n.parent.color == red && n.color == red {
			return errConsecutiveRed
		}

		if isLeftOf(n.parent, n) &&
			event.PickAggregateVersion(n.parent.evt) < event.PickAggregateVersion(n.evt) {
			return errInvalidOrder
		}

		if isRightOf(n.parent, n) &&
			event.PickAggregateVersion(n.parent.evt) >= event.PickAggregateVersion(n.evt) {
			return errInvalidOrder
		}
	}

	if err := n.left.validate(); err != nil {
		return err
	}

	return n.right.validate()
}

func (n *node) level(l int) []*node {
	if n == nil {
		return nil
	}
	if l <= 0 {
		return []*node{n}
	}
	if n.left == nil && n.right == nil {
		return nil
	}

	if n.left == nil {
		return n.right.level(l - 1)
	}

	if n.right == nil {
		return n.left.level(l - 1)
	}

	return append(n.left.level(l-1), n.right.level(l-1)...)
}

func (n *node) insert(evt event.Event) *node {
	v := event.PickAggregateVersion(evt)
	if v <= event.PickAggregateVersion(n.evt) {
		return n.insertLeft(evt)
	}
	return n.insertRight(evt)
}

func (n *node) insertLeft(evt event.Event) *node {
	if n.left == nil {
		n.left = &node{evt: evt, parent: n}
		return n.left
	}
	return n.left.insert(evt)
}

func (n *node) insertRight(evt event.Event) *node {
	if n.right == nil {
		n.right = &node{evt: evt, parent: n}
		return n.right
	}
	return n.right.insert(evt)
}

func (n *node) walk(fn func(*node)) {
	if n.left != nil {
		n.left.walk(fn)
	}
	fn(n)
	if n.right != nil {
		n.right.walk(fn)
	}
}

func (n *node) grandparent() *node {
	if n.parent == nil {
		return nil
	}
	return n.parent.parent
}

func (n *node) uncle() *node {
	if n.parent == nil {
		return nil
	}
	return n.parent.sibling()
}

func (n *node) sibling() *node {
	if n.parent == nil {
		return nil
	}
	if n.parent.left == n {
		return n.parent.right
	}
	return n.parent.left
}

func (n *node) maxDepth() int {
	if n.left == nil && n.right == nil {
		var d int
		for n.parent != nil {
			d++
			n = n.parent
		}
		return d
	}

	left, right := 0, 0
	if n.left != nil {
		left = n.left.maxDepth()
	}
	if n.right != nil {
		right = n.right.maxDepth()
	}

	return int(math.Max(float64(left), float64(right)))
}

func (n *node) setLeft(l *node) {
	n.left = l
	l.parent = n
}

func (n *node) setRight(r *node) {
	n.right = r
	r.parent = n
}

func (c color) String() string {
	switch c {
	case black:
		return "black"
	case red:
		return "red"
	default:
		return "<invalid>"
	}
}

func isLeftOf(p, n *node) bool {
	return p != nil && n != nil && p.left == n
}

func isRightOf(p, n *node) bool {
	return p != nil && n != nil && p.right == n
}
