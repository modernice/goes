package etree

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xevent"
)

// when rotating `4`
//
//                6
//           4 ───┴─── 7
//      2 ───┴─── 5
// 1 ───┴─── 3
//
// should become
//
//                     6
//                5 ───┴─── 7
//           4 ───┘
//      2 ───┘
// 1 ───┴─── 3
func TestTree_rotateLeft(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 7, xevent.ForAggregate(a))

	nodes := make([]*node, 7)
	for i := range nodes {
		nodes[i] = &node{evt: events[i]}
	}

	nodes[5].setLeft(nodes[3])
	nodes[5].setRight(nodes[6])
	nodes[3].setLeft(nodes[1])
	nodes[3].setRight(nodes[4])
	nodes[1].setLeft(nodes[0])
	nodes[1].setRight(nodes[2])

	var tr Tree
	tr.root = nodes[5]

	tr.rotateLeft(nodes[3])

	assertIntMatrix(t, [][]int{
		{6},
		{5, 7},
		{4},
		{2},
		{1, 3},
	}, toIntMatrix(tr.Matrix()))
}

// when rotating `4`
//
//                6
//           4 ───┴─── 7
//      2 ───┴─── 5
// 1 ───┴─── 3
//
// should become
//
//                6
//           2 ───┴─── 7
//      1 ───┴─── 4
//           3 ───┴─── 5
func TestTree_rotateRight(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 7, xevent.ForAggregate(a))

	nodes := make([]*node, 7)
	for i := range nodes {
		nodes[i] = &node{evt: events[i]}
	}

	nodes[5].setLeft(nodes[3])
	nodes[5].setRight(nodes[6])
	nodes[3].setLeft(nodes[1])
	nodes[3].setRight(nodes[4])
	nodes[1].setLeft(nodes[0])
	nodes[1].setRight(nodes[2])

	var tr Tree
	tr.root = nodes[5]

	tr.rotateRight(nodes[3])

	assertIntMatrix(t, [][]int{
		{6},
		{2, 7},
		{1, 4},
		{3, 5},
	}, toIntMatrix(tr.Matrix()))
}

func TestTree_Insert(t *testing.T) {
	var tr Tree
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 100, xevent.ForAggregate(a))
	events = xevent.Shuffle(events)

	for _, evt := range events {
		tr.Insert(evt)
	}

	var walked []event.Event[any]
	tr.Walk(func(evt event.Event[any]) {
		walked = append(walked, evt)
	})

	test.AssertEqualEvents(
		t,
		event.Sort(events, event.SortAggregateVersion, event.SortAsc),
		walked,
	)
}

func TestTree_Insert_root_color(t *testing.T) {
	var tr Tree
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 1, xevent.ForAggregate(a))

	tr.Insert(events[0])

	if tr.root == nil {
		t.Fatalf("tr.root should not be <nil>")
	}

	if tr.root.evt != events[0] {
		t.Errorf("tr.root.evt should be %#v; got %#v", events[0], tr.root.evt)
	}

	if tr.root.color != black {
		t.Errorf("inserted root node should be black; is %s", tr.root.color)
	}
}

func TestTree_Insert_finalize(t *testing.T) {
	var tr Tree
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 32, xevent.ForAggregate(a))
	events = xevent.Shuffle(events)
	// sorted := event.Sort(events, event.SortAggregateVersion, event.SortAsc)

	// insert root node
	tr.Insert(events[0])

	for i, evt := range events[1:] {
		tr.Insert(evt)

		var inserted *node
		tr.walk(func(n *node) {
			if n.evt.ID() != evt.ID() {
				return
			}
			inserted = n
		})

		if inserted == nil {
			t.Fatalf("[%d] could not find node with the correct event", i)
		}

		if err := tr.validate(); err != nil {
			t.Errorf("[%d] tree validation failed: %v", i, err)
		}
	}
}

//                         4B
//           2B ───────────┴─────────── 7R
// 1R ───────┴─────── 3R      6B ───────┴─────── 8B
//              5R ───┘
func TestTree_Matrix(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{A: "foo"}, 8, xevent.ForAggregate(a))
	events = event.Sort(events, event.SortAggregateVersion, event.SortAsc)

	root := &node{evt: events[3], color: black}
	n2 := &node{evt: events[1], color: black, parent: root}
	n7 := &node{evt: events[6], color: red, parent: root}
	root.left = n2
	root.right = n7

	n1 := &node{evt: events[0], color: red, parent: n2}
	n2.left = n1

	n3 := &node{evt: events[2], color: red, parent: n2}
	n2.right = n3

	n6 := &node{evt: events[5], color: black, parent: n7}
	n7.left = n6

	n8 := &node{evt: events[7], color: black, parent: n7}
	n7.right = n8

	n5 := &node{evt: events[4], color: red, parent: n6}
	n6.left = n5

	tr := Tree{root: root}

	mx := tr.Matrix()

	assertMatrix(t, [][]event.Event[any]{
		{root.evt},
		{n2.evt, n7.evt},
		{n1.evt, n3.evt, n6.evt, n8.evt},
		{n5.evt},
	}, mx)
}

func TestTree_validate(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(a))

	tests := []struct {
		name   string
		insert func(*Tree)
		want   error
	}{
		{
			name:   "empty",
			insert: func(*Tree) {},
		},
		{
			name: "1B",
			insert: func(t *Tree) {
				t.Insert(events[0]) // 1B
			},
		},
		{
			name: "2B -> 1R",
			insert: func(t *Tree) {
				//       2B
				// 1R ───┘
				t.Insert(events[1])
				n1 := &node{evt: events[0], color: red}
				t.root.setLeft(n1)
			},
		},
		{
			name: "2B -> 1R / 3R",
			insert: func(t *Tree) {
				//       2B
				// 1R ───┴─── 3R
				t.Insert(events[1])
				n1 := &node{evt: events[0], color: red}
				t.root.setLeft(n1)
				n3 := &node{evt: events[2], color: red}
				t.root.setRight(n3)
			},
		},
		{
			name: "3B -> 2R / 4R -> 1B",
			insert: func(t *Tree) {
				//           3B
				//     2R ───┴─── 4R
				// ┌───┘
				// 1B
				t.Insert(events[2])
				n2 := &node{evt: events[1], color: red}
				n4 := &node{evt: events[3], color: red}
				n1 := &node{evt: events[0], color: black}
				t.root.setLeft(n2)
				t.root.setRight(n4)
				n2.setLeft(n1)
			},
		},
		{
			name: "3B -> 2R / 4R -> 1R",
			insert: func(t *Tree) {
				//           3B
				//     2R ───┴─── 4R
				// ┌───┘
				// 1R
				t.Insert(events[2])
				n2 := &node{evt: events[1], color: red}
				n4 := &node{evt: events[3], color: red}
				n1 := &node{evt: events[0], color: red}
				t.root.setLeft(n2)
				t.root.setRight(n4)
				n2.setLeft(n1)
			},
			want: errConsecutiveRed,
		},
		{
			name: "3B -> 1R / 4R -> 2B",
			insert: func(t *Tree) {
				//             3B
				//       1R ───┴─── 4R
				// 2B ───┘
				t.Insert(events[2])
				n1 := &node{evt: events[0], color: red}
				n4 := &node{evt: events[3], color: red}
				n2 := &node{evt: events[1], color: black}
				t.root.setLeft(n1)
				t.root.setRight(n4)
				n1.setLeft(n2)
			},
			want: errInvalidOrder,
		},
		{
			name: "3B -> 2R / 4R -> 1B / 5B",
			insert: func(t *Tree) {
				//             3B
				//       2R ───┴─── 4R
				// 1B ───┘          └─── 5B
				t.Insert(events[2])
				n1 := &node{evt: events[0], color: black}
				n4 := &node{evt: events[3], color: red}
				n5 := &node{evt: events[4], color: black}
				n2 := &node{evt: events[1], color: red}
				t.root.setLeft(n2)
				t.root.setRight(n4)
				n2.setLeft(n1)
				n4.setRight(n5)
			},
		},
		{
			name: "3B -> 2R / 5R -> 1B / 4B",
			insert: func(t *Tree) {
				//             3B
				//       2R ───┴─── 5R
				// 1B ───┘          └─── 4B
				t.Insert(events[2])
				n1 := &node{evt: events[0], color: black}
				n2 := &node{evt: events[1], color: red}
				n4 := &node{evt: events[3], color: black}
				n5 := &node{evt: events[4], color: red}
				t.root.setLeft(n2)
				t.root.setRight(n5)
				n2.setLeft(n1)
				n5.setRight(n4)
			},
			want: errInvalidOrder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tr Tree
			tt.insert(&tr)

			if err := tr.validate(); !errors.Is(err, tt.want) {
				t.Errorf("expected validation error %#v; got %#v", tt.want, err)
			}
		})
	}
}

func TestTree_Size(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, 100, xevent.ForAggregate(a))

	var tr Tree
	for i, evt := range events {
		tr.Insert(evt)
		if s := tr.Size(); s != i+1 {
			t.Errorf("tree should have size %d; got %d", i+1, s)
		}
	}
}

func toIntMatrix(mx [][]event.Event[any]) [][]int {
	ix := make([][]int, len(mx))
	for i, events := range mx {
		ix[i] = make([]int, len(events))
		for j := range events {
			ix[i][j] = event.PickAggregateVersion(events[j])
		}
	}
	return ix
}

func assertMatrix(t *testing.T, want, got [][]event.Event[any]) {
	if len(want) != len(got) {
		t.Errorf("returned matrix has wrong len. want %d; got %d", len(want), len(got))
	}

	for i, wr := range want {
		gr := got[i]
		if len(gr) != len(wr) {
			t.Errorf("matrix row has wrong len. want %d; got %d", len(wr), len(gr))
			continue
		}

		for j, w := range wr {
			g := gr[j]
			if w != g {
				t.Errorf("element %d:%d should be\n\n%#v\n\ngot %#v\n\n", i, j, w, g)
			}
		}
	}
}

func assertIntMatrix(t *testing.T, want, got [][]int) {
	if len(want) != len(got) {
		t.Errorf("returned matrix has wrong len. want %d; got %d", len(want), len(got))
	}

	for i, wr := range want {
		gr := got[i]
		if len(gr) != len(wr) {
			t.Errorf("matrix row has wrong len. want %d; got %d", len(wr), len(gr))
			continue
		}

		for j, w := range wr {
			g := gr[j]
			if w != g {
				t.Errorf("element %d:%d should be\n\n%#v\n\ngot %#v\n\n", i, j, w, g)
			}
		}
	}
}
