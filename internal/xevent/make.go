package xevent

import (
	"time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// MakeOption is an option for Make.
type MakeOption[ID goes.ID] func(*makeConfig[ID])

type makeConfig[ID goes.ID] struct {
	as   []aggregate.AggregateOf[ID]
	skip []int
}

// ForAggregate returns a MakeOption that sets the aggregate for events returne
// by Make.
func ForAggregate[ID goes.ID](as ...aggregate.AggregateOf[ID]) MakeOption[ID] {
	return func(cfg *makeConfig[ID]) {
		cfg.as = append(cfg.as, as...)
	}
}

// SkipVersion returns a MakeOption that emulates a consistency error by
// skipping the specified aggregate versions when creating the events. This
// option only has an effect when used together with ForAggregate.
func SkipVersion[ID goes.ID](v ...int) MakeOption[ID] {
	return func(cfg *makeConfig[ID]) {
		cfg.skip = append(cfg.skip, v...)
	}
}

// Make returns n Events with the specified name and data.
func Make[ID goes.ID](newID func() ID, name string, data any, n int, opts ...MakeOption[ID]) []event.Of[any, ID] {
	var cfg makeConfig[ID]
	for _, opt := range opts {
		opt(&cfg)
	}

	if len(cfg.as) == 0 {
		return makeEvents(newID, name, data, n)
	}

	return makeAggregateEvents(newID, name, data, n, cfg)
}

func (cfg makeConfig[ID]) aggregateVersion(b, i int, skipped *int) int {
	v := b + i + 1
	for _, skip := range cfg.skip {
		if skip == v {
			*skipped++
		}
	}
	v += *skipped
	return v
}

func makeEvents[ID goes.ID](newID func() ID, name string, data any, n int) []event.Of[any, ID] {
	events := make([]event.Of[any, ID], n)
	for i := range events {
		events[i] = event.New(newID(), name, data)
	}
	return events
}

func makeAggregateEvents[ID goes.ID](newID func() ID, name string, data any, n int, cfg makeConfig[ID]) []event.Of[any, ID] {
	events := make([]event.Of[any, ID], 0, n*len(cfg.as))
	for _, a := range cfg.as {
		var skipped int
		aevents := make([]event.Of[any, ID], n)
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
			aevents[i] = event.New(newID(), name, data, opts...)
			t = t.Add(time.Nanosecond)
		}
		events = append(events, aevents...)
	}
	return events
}
