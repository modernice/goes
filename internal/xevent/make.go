package xevent

import (
	"time"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// MakeOption is an option for Make.
type MakeOption func(*makeConfig)

type makeConfig struct {
	as   []aggregate.Aggregate
	skip []int
}

// ForAggregate returns a MakeOption that sets the aggregate for events returne
// by Make.
func ForAggregate(as ...aggregate.Aggregate) MakeOption {
	return func(cfg *makeConfig) {
		cfg.as = append(cfg.as, as...)
	}
}

// SkipVersion returns a MakeOption that emulates a consistency error by
// skipping the specified aggregate versions when creating the events. This
// option only has an effect when used together with ForAggregate.
func SkipVersion(v ...int) MakeOption {
	return func(cfg *makeConfig) {
		cfg.skip = append(cfg.skip, v...)
	}
}

// Make returns n events with the specified name and data.
func Make(name string, data any, n int, opts ...MakeOption) []event.Event {
	var cfg makeConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if len(cfg.as) == 0 {
		return makeEvents(name, data, n)
	}

	return makeAggregateEvents(name, data, n, cfg)
}

func (cfg makeConfig) aggregateVersion(b, i int, skipped *int) int {
	v := b + i + 1
	for _, skip := range cfg.skip {
		if skip == v {
			*skipped++
		}
	}
	v += *skipped
	return v
}

func makeEvents(name string, data any, n int) []event.Event {
	events := make([]event.Event, n)
	for i := range events {
		events[i] = event.New(name, data)
	}
	return events
}

func makeAggregateEvents(name string, data any, n int, cfg makeConfig) []event.Event {
	events := make([]event.Event, 0, n*len(cfg.as))
	for _, a := range cfg.as {
		var skipped int
		aevents := make([]event.Event, n)
		t := time.Now()

		aid, aname, av := a.Aggregate()

		for i := range aevents {
			var opts []event.Option
			v := cfg.aggregateVersion(av, i, &skipped)
			opts = append(opts, event.Aggregate(
				aid,
				aname,
				v,
			), event.Time(t))
			aevents[i] = event.New(name, data, opts...)
			t = t.Add(time.Nanosecond)
		}
		events = append(events, aevents...)
	}
	return events
}
