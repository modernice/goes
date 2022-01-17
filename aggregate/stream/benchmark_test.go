package stream_test

import (
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/stream"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xtime"
)

var names = [...]string{
	"foo", "bar", "baz", "foobar",
	"foobaz", "bazbar", "barbaz",
}

type mockAggregate struct {
	*aggregate.Base[any]

	a float64
	b string
}

func BenchmarkStream_10A_1E(b *testing.B) {
	benchmark(b, 10, 1)
}

func BenchmarkStream_10A_10E(b *testing.B) {
	benchmark(b, 10, 10)
}

func BenchmarkStream_10A_100E(b *testing.B) {
	benchmark(b, 10, 100)
}

func BenchmarkStream_10A_1000E(b *testing.B) {
	benchmark(b, 10, 1000)
}

func BenchmarkStream_10A_10000E(b *testing.B) {
	benchmark(b, 10, 10000)
}

func BenchmarkStream_10A_100000E(b *testing.B) {
	benchmark(b, 10, 100000)
}

func BenchmarkStream_100A_1E(b *testing.B) {
	benchmark(b, 100, 1)
}

func BenchmarkStream_100A_10E(b *testing.B) {
	benchmark(b, 100, 10)
}

func BenchmarkStream_100A_100E(b *testing.B) {
	benchmark(b, 100, 100)
}

func BenchmarkStream_100A_1000E(b *testing.B) {
	benchmark(b, 100, 1000)
}

func BenchmarkStream_100A_10000E(b *testing.B) {
	benchmark(b, 100, 10000)
}

func BenchmarkStream_1000A_1E(b *testing.B) {
	benchmark(b, 1000, 1)
}

func BenchmarkStream_1000A_10E(b *testing.B) {
	benchmark(b, 1000, 10)
}

func BenchmarkStream_1000A_100E(b *testing.B) {
	benchmark(b, 1000, 100)
}

func BenchmarkStream_1000A_1000E(b *testing.B) {
	benchmark(b, 1000, 1000)
}

func BenchmarkStream_10000A_1E(b *testing.B) {
	benchmark(b, 10000, 1)
}

func BenchmarkStream_10000A_10E(b *testing.B) {
	benchmark(b, 10000, 10)
}

func BenchmarkStream_10000A_100E(b *testing.B) {
	benchmark(b, 10000, 100)
}

func BenchmarkStream_100000A_1E(b *testing.B) {
	benchmark(b, 100000, 1)
}

func BenchmarkStream_100000A_10E(b *testing.B) {
	benchmark(b, 100000, 10)
}

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
	var opts []stream.Option[any]
	if grouped {
		opts = append(opts, stream.Grouped[any](true))
	}
	if sorted {
		opts = append(opts, stream.Sorted[any](true))
	}

	b.ReportAllocs()
	b.ResetTimer()

	var gerr error
L:
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		var as []aggregate.Aggregate[any]
		estr := event.Stream(events...)
		b.StartTimer()

		str, errs := stream.New(estr, opts...)
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
				a := &mockAggregate{Base: aggregate.New[any](res.AggregateName(), res.AggregateID())}
				as = append(as, a)
			}
		}
	}
	_ = gerr
}

func (a *mockAggregate) ApplyEvent(evt event.Event[any]) {
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

func makeAggregates(n int) []aggregate.Aggregate[any] {
	as := make([]aggregate.Aggregate[any], n)
	for i := range as {
		name := randomName()
		as[i] = aggregate.New[any](name, uuid.New())
	}
	return as
}

func makeEvents(n int, as []aggregate.Aggregate[any], grouped, sorted bool) []event.Event[any] {
	rand.Seed(xtime.Now().UnixNano())
	eventm := make(map[aggregate.Aggregate[any]][]event.Event[any])
	for _, a := range as {
		events := make([]event.Event[any], n)
		for i := range events {
			id, name, v := a.Aggregate()
			v += i + 1
			evt := event.New[any](
				randomName(),
				test.FooEventData{},
				event.Aggregate[any](id, name, v),
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
	out := make([]event.Event[any], 0, len(as)*n)
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
