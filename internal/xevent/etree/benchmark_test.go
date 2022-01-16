package etree_test

import (
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xevent"
	"github.com/modernice/goes/internal/xevent/etree"
)

var tmpStack []event.Event[any]

func BenchmarkTree_Insert10(b *testing.B) { benchmarkTree_Insert(b, 10) }

func BenchmarkTree_Insert100(b *testing.B) { benchmarkTree_Insert(b, 100) }

func BenchmarkTree_Insert1000(b *testing.B) { benchmarkTree_Insert(b, 1000) }

func BenchmarkTree_Insert10000(b *testing.B) { benchmarkTree_Insert(b, 10000) }

func BenchmarkTree_Insert100000(b *testing.B) { benchmarkTree_Insert(b, 100000) }

func BenchmarkTree_Insert1000000(b *testing.B) { benchmarkTree_Insert(b, 1000000) }

func BenchmarkTree_Insert5000000(b *testing.B) { benchmarkTree_Insert(b, 5000000) }

func benchmarkTree_Insert(b *testing.B, n int) {
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, n, xevent.ForAggregate(a))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		events = xevent.Shuffle(events)

		var tr etree.Tree
		for _, evt := range events {
			tr.Insert(evt)
		}

		walked := make([]event.Event[any], 0, tr.Size())
		tr.Walk(func(evt event.Event[any]) {
			walked = append(walked, evt)
		})
		tmpStack = walked
	}
}

func BenchmarkSlice_Insert10(b *testing.B) { benchmarkSlice_Insert(b, 10) }

func BenchmarkSlice_Insert100(b *testing.B) { benchmarkSlice_Insert(b, 100) }

func BenchmarkSlice_Insert1000(b *testing.B) { benchmarkSlice_Insert(b, 1000) }

func BenchmarkSlice_Insert10000(b *testing.B) { benchmarkSlice_Insert(b, 10000) }

func BenchmarkSlice_Insert100000(b *testing.B) { benchmarkSlice_Insert(b, 100000) }

func BenchmarkSlice_Insert1000000(b *testing.B) { benchmarkSlice_Insert(b, 1000000) }

func BenchmarkSlice_Insert5000000(b *testing.B) { benchmarkSlice_Insert(b, 5000000) }

func benchmarkSlice_Insert(b *testing.B, n int) {
	a := aggregate.New("foo", uuid.New())
	events := xevent.Make("foo", test.FooEventData{}, n, xevent.ForAggregate(a))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		events = xevent.Shuffle(events)
		var stack []event.Event[any]
		for _, evt := range events {
			stack = stackInsert(stack, evt, false)
		}
		sortStack(stack)
		tmpStack = stack
	}
}

func stackInsert(stack []event.Event[any], evt event.Event[any], s bool) []event.Event[any] {
	stack = append(stack, evt)
	if s {
		sortStack(stack)
	}
	return stack
}

func sortStack(stack []event.Event[any]) {
	sort.Slice(stack, func(i, j int) bool {
		return event.PickAggregateVersion(stack[i]) <= event.PickAggregateVersion(stack[j])
	})
}
