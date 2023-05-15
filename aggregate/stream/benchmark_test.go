package stream_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/xtime"
)

var names = [...]string{
	"foo", "bar", "baz", "foobar",
	"foobaz", "bazbar", "barbaz",
}

type mockAggregate struct {
	*aggregate.Base

	a float64
	b string
}

// BenchmarkStream_10A_1E benchmarks the streaming of events for 10 aggregates
// with 1 event per aggregate.
func BenchmarkStream_10A_1E(b *testing.B) {
	benchmark(b, 10, 1)
}

// BenchmarkStream_10A_10E runs a benchmark for streaming 10 events for each of
// 10 aggregates with various options such as grouping and sorting.
func BenchmarkStream_10A_10E(b *testing.B) {
	benchmark(b, 10, 10)
}

// BenchmarkStream_10A_100E benchmarks the performance of streaming 100 events
// for 10 aggregates with various options such as ungrouped and unsorted,
// grouped and unsorted, or grouped and sorted.
func BenchmarkStream_10A_100E(b *testing.B) {
	benchmark(b, 10, 100)
}

// BenchmarkStream_10A_1000E benchmarks the stream performance with 10
// aggregates and 1000 events per aggregate. The benchmark runs ungrouped and
// unsorted, grouped and unsorted, and grouped and sorted scenarios.
func BenchmarkStream_10A_1000E(b *testing.B) {
	benchmark(b, 10, 1000)
}

// BenchmarkStream_10A_10000E benchmarks the streaming of events for 10
// aggregates with 10000 events per aggregate, with various grouping and sorting
// options.
func BenchmarkStream_10A_10000E(b *testing.B) {
	benchmark(b, 10, 10000)
}

// BenchmarkStream_10A_100000E benchmarks the stream function with 10 aggregates
// and 100,000 events.
func BenchmarkStream_10A_100000E(b *testing.B) {
	benchmark(b, 10, 100000)
}

// BenchmarkStream_100A_1E benchmarks the streaming of events for 100 aggregates
// with 1 event per aggregate.
func BenchmarkStream_100A_1E(b *testing.B) {
	benchmark(b, 100, 1)
}

// BenchmarkStream_100A_10E benchmarks the performance of streaming 10 events
// for 100 aggregates with various grouping and sorting options.
func BenchmarkStream_100A_10E(b *testing.B) {
	benchmark(b, 100, 10)
}

// BenchmarkStream_100A_100E benchmarks the performance of streaming 100 events
// for 100 aggregates with various options such as ungrouped and unsorted,
// grouped and unsorted, or grouped and sorted.
func BenchmarkStream_100A_100E(b *testing.B) {
	benchmark(b, 100, 100)
}

// BenchmarkStream_100A_1000E benchmarks the stream performance with 100
// aggregates and 1000 events per aggregate, with various grouping and sorting
// scenarios.
func BenchmarkStream_100A_1000E(b *testing.B) {
	benchmark(b, 100, 1000)
}

// BenchmarkStream_100A_10000E benchmarks the streaming of events for 100
// aggregates with 10,000 events per aggregate with various grouping and sorting
// options.
func BenchmarkStream_100A_10000E(b *testing.B) {
	benchmark(b, 100, 10000)
}

// BenchmarkStream_1000A_1E benchmarks the streaming of events for 1000
// aggregates with 1 event per aggregate, with options for ungrouped and
// unsorted, grouped and unsorted, and grouped and sorted streams.
func BenchmarkStream_1000A_1E(b *testing.B) {
	benchmark(b, 1000, 1)
}

// BenchmarkStream_1000A_10E benchmarks the streaming of 10 events for 1000
// aggregates with various grouping and sorting options.
func BenchmarkStream_1000A_10E(b *testing.B) {
	benchmark(b, 1000, 10)
}

// BenchmarkStream_1000A_100E benchmarks the performance of streaming 100 events
// for 1000 aggregates with various options such as ungrouped and unsorted,
// grouped and unsorted, or grouped and sorted.
func BenchmarkStream_1000A_100E(b *testing.B) {
	benchmark(b, 1000, 100)
}

// BenchmarkStream_1000A_1000E benchmarks the performance of streaming 1000
// events for 1000 aggregates with various options such as ungrouped and
// unsorted, grouped and unsorted, or grouped and sorted.
func BenchmarkStream_1000A_1000E(b *testing.B) {
	benchmark(b, 1000, 1000)
}

// BenchmarkStream_10000A_1E benchmarks the streaming of one event for 10,000
// aggregates with various grouping and sorting options.
func BenchmarkStream_10000A_1E(b *testing.B) {
	benchmark(b, 10000, 1)
}

// BenchmarkStream_10000A_10E benchmarks the streaming of events for 10,000
// aggregates with 10 events per aggregate, with various grouping and sorting
// options.
func BenchmarkStream_10000A_10E(b *testing.B) {
	benchmark(b, 10000, 10)
}

// BenchmarkStream_10000A_100E benchmarks the performance of streaming 100
// events for 10,000 aggregates with various grouping and sorting options.
func BenchmarkStream_10000A_100E(b *testing.B) {
	benchmark(b, 10000, 100)
}

// BenchmarkStream_100000A_1E measures the performance of streaming 1 event for
// 100000 different aggregates with options for grouped and sorted streams.
func BenchmarkStream_100000A_1E(b *testing.B) {
	benchmark(b, 100000, 1)
}

// BenchmarkStream_100000A_10E benchmarks the performance of streaming 10 events
// for 100,000 different aggregates, with options for grouped and sorted
// streams.
func BenchmarkStream_100000A_10E(b *testing.B) {
	benchmark(b, 100000, 10)
}

// BenchmarkStream_100000A_100E measures the performance of streaming 100000
// aggregates with 100 events each.
func BenchmarkStream_100000A_100E(b *testing.B) {
	benchmark(b, 100000, 100)
}

func benchmark(b *testing.B, naggregates, nevents int) {
	b.Run("Ungrouped+Unsorted", func(b *testing.B) {
		run(b, naggregates, nevents, false, false)
	})

	b.Run("Grouped+Unsorted", func(b *testing.B) {
		run(b, naggregates, nevents, true, false)
	})

	b.Run("Grouped+Sorted", func(b *testing.B) {
		run(b, naggregates, nevents, true, true)
	})
}

func run(b *testing.B, naggregates, nevents int, grouped, sorted bool) {
	as := makeAggregates(naggregates)
	events := makeEvents(nevents, as, grouped, sorted)
	var opts []stream.Option
	if grouped {
		opts = append(opts, stream.Grouped(true))
	}
	if sorted {
		opts = append(opts, stream.Sorted(true))
	}

	b.ReportAllocs()
	b.ResetTimer()

	var gerr error
L:
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		var as []aggregate.Aggregate
		estr := streams.New(events)
		b.StartTimer()

		str, errs := stream.New(context.Background(), estr, opts...)
		for {
			select {
			case err, ok := <-errs:
				if !ok {
					continue L
				}
				gerr = err
			case res, ok := <-str:
				if !ok {
					continue L
				}

				ref := res.Aggregate()

				a := &mockAggregate{Base: aggregate.New(ref.Name, ref.ID)}
				as = append(as, a)
			}
		}
	}
	_ = gerr
}

// ApplyEvent applies an event to the mockAggregate. It updates the state of the
// mockAggregate according to the event's name.
func (a *mockAggregate) ApplyEvent(evt event.Event) {
	for i, name := range names {
		if name != evt.Name() {
			continue
		}
		if i%2 == 0 {
			a.a += float64(i)
			a.a *= 1.5
			continue
		}
		a.b += a.b
	}
}

func makeAggregates(n int) []aggregate.Aggregate {
	as := make([]aggregate.Aggregate, n)
	for i := range as {
		name := randomName()
		as[i] = aggregate.New(name, uuid.New())
	}
	return as
}

func makeEvents(n int, as []aggregate.Aggregate, grouped, sorted bool) []event.Event {
	rand.Seed(xtime.Now().UnixNano())
	eventm := make(map[aggregate.Aggregate][]event.Event)
	for _, a := range as {
		events := make([]event.Event, n)
		for i := range events {
			id, name, v := a.Aggregate()
			v += i + 1
			evt := event.New[any](
				randomName(),
				test.FooEventData{},
				event.Aggregate(id, name, v),
			)
			events[i] = evt
		}
		if !sorted {
			rand.Shuffle(len(events), func(i, j int) {
				events[i], events[j] = events[j], events[i]
			})
		}
		eventm[a] = events
	}
	out := make([]event.Event, 0, len(as)*n)
	for _, events := range eventm {
		out = append(out, events...)
	}
	if !grouped {
		rand.Shuffle(len(out), func(i, j int) {
			out[i], out[j] = out[j], out[i]
		})
	}
	return out
}

func randomName() string {
	return names[rand.Intn(7)]
}
