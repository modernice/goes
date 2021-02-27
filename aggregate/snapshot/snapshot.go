package snapshot

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// Snapshot is a snapshot of an Aggregate.
type Snapshot interface {
	// AggregateName returns the name of the Aggregate.
	AggregateName() string

	// AggregateID returns the UUID of the Aggregate.
	AggregateID() uuid.UUID

	// AggregateVersion returns the version of the Aggregate.
	AggregateVersion() int

	// Time returns the Time of the Snapshot.
	Time() time.Time

	// Data returns the encoded data of the Aggregate.
	Data() []byte
}

// Option is a Snapshot option.
type Option func(*snapshot)

type snapshot struct {
	aggregate.Aggregate

	time time.Time
	data []byte
}

// Time returns an Option that sets the Time of a Snapshot.
func Time(t time.Time) Option {
	return func(s *snapshot) {
		s.time = t
	}
}

// Data returns an Option that overrides the encoded data of a Snapshot.
func Data(b []byte) Option {
	return func(s *snapshot) {
		s.data = b
	}
}

// New creates and returns a Snapshot of the given Aggregate.
func New(a aggregate.Aggregate, opts ...Option) (Snapshot, error) {
	snap := snapshot{
		Aggregate: a,
		time:      time.Now(),
	}
	for _, opt := range opts {
		opt(&snap)
	}
	if snap.data == nil {
		b, err := Marshal(a)
		if err != nil {
			return nil, fmt.Errorf("marshal snapshot: %w", err)
		}
		snap.data = b
	}
	return &snap, nil
}

func (s *snapshot) Time() time.Time {
	return s.time
}

func (s *snapshot) Data() []byte {
	return s.data
}
