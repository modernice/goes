package test

import (
	"fmt"
	"reflect"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// ExpectedChangeError is returned by the `Change` testing helper when the
// testd Aggregate doesn't have the required change.
type ExpectedChangeError struct {
	// EventName is the name of the tested change.
	EventName string

	// Matches is the number of changes that matched.
	Matches int

	cfg changeConfig
}

func (err ExpectedChangeError) Error() string {
	if err.cfg.atLeast > 0 && err.Matches < err.cfg.atLeast {
		return fmt.Sprintf("expected at least %d %q changes; got %d", err.cfg.atLeast, err.EventName, err.Matches)
	}

	if err.cfg.atMost > 0 && err.Matches > err.cfg.atMost {
		return fmt.Sprintf("expected at most %d %q changes; got %d", err.cfg.atMost, err.EventName, err.Matches)
	}

	if err.cfg.exactly > 0 && err.Matches != err.cfg.exactly {
		return fmt.Sprintf("expected exactly %d %q changes; got %d", err.cfg.exactly, err.EventName, err.Matches)
	}

	return fmt.Sprintf("expected %q change", err.EventName)
}

// UnexpectedChangeError is returned by the `NoChange` testing helper when the
// testd Aggregate does have an unwanted change.
type UnexpectedChangeError struct {
	// EventName is the name of the tested change.
	EventName string
}

func (err UnexpectedChangeError) Error() string {
	return fmt.Sprintf("unexpected %q change", err.EventName)
}

// ChangeOption is an option for the `Change` testing helper.
type ChangeOption func(*changeConfig)

type changeConfig struct {
	eventData event.Data
	atLeast   int
	atMost    int
	exactly   int
}

// WithEventData returns a ChangeOption that also tests the event data of
// changes instead of just the event name.
func WithEventData(d event.Data) ChangeOption {
	return func(cfg *changeConfig) {
		cfg.eventData = d
	}
}

// AtLeast returns a ChangeOption that requires an Aggregate to have a change at
// least as many times as provided.
//
// AtLeast has no effect when used in `NoChange`.
func AtLeast(times int) ChangeOption {
	return func(cfg *changeConfig) {
		cfg.atLeast = times
	}
}

// AtMost returns a ChangeOption that requires an Aggregate to have a change at
// most as many times as provided.
//
// AtMost has no effect when used in `NoChange`.
func AtMost(times int) ChangeOption {
	return func(cfg *changeConfig) {
		cfg.atMost = times
	}
}

// Exactly returns a ChangeOption that requires an Aggregate to have a change
// exactly as many times as provided.
//
// Exactly has no effect when used in `NoChange`.
func Exactly(times int) ChangeOption {
	return func(cfg *changeConfig) {
		cfg.exactly = times
	}
}

// Change tests an Aggregate for a change. The Aggregate must have an
// uncommitted change with the specified event name.
func Change(t TestingT, a aggregate.Aggregate, eventName string, opts ...ChangeOption) {
	var cfg changeConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var matches int
	for _, change := range a.AggregateChanges() {
		if change.Name() != eventName {
			continue
		}

		if cfg.eventData != nil && !reflect.DeepEqual(cfg.eventData, change.Data()) {
			continue
		}

		matches++
	}

	if cfg.atLeast > 0 && matches < cfg.atLeast {
		t.Fatal(&ExpectedChangeError{
			EventName: eventName,
			Matches:   matches,
			cfg:       cfg,
		})
		return
	}

	if cfg.atMost > 0 && matches > cfg.atMost {
		t.Fatal(&ExpectedChangeError{
			EventName: eventName,
			Matches:   matches,
			cfg:       cfg,
		})
		return
	}

	if matches == 0 || (cfg.exactly > 0 && matches != cfg.exactly) {
		t.Fatal(&ExpectedChangeError{
			EventName: eventName,
			Matches:   matches,
			cfg:       cfg,
		})
	}
}

// Change tests an Aggregate for a change. The Aggregate must not have an
// uncommitted change with the specified event name.
func NoChange(t TestingT, a aggregate.Aggregate, eventName string, opts ...ChangeOption) {
	var cfg changeConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var matches int
	for _, change := range a.AggregateChanges() {
		if change.Name() != eventName {
			continue
		}

		if cfg.eventData != nil && !reflect.DeepEqual(cfg.eventData, change.Data()) {
			continue
		}

		matches++
	}

	if matches != 0 {
		t.Fatal(&UnexpectedChangeError{
			EventName: eventName,
		})
	}
}
