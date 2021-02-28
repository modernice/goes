package snapshot

import (
	"fmt"
	"sort"
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

// Sort sorts Snapshot and returns the sorted Snapshots.
func Sort(snaps []Snapshot, s aggregate.Sorting, dir aggregate.SortDirection) []Snapshot {
	return SortMulti(snaps, aggregate.SortOptions{Sort: s, Dir: dir})
}

// SortMulti sorts Snapshots by multiple fields and returns the sorted
// aggregates.
func SortMulti(snaps []Snapshot, sorts ...aggregate.SortOptions) []Snapshot {
	sorted := make([]Snapshot, len(snaps))
	copy(sorted, snaps)

	sort.Slice(sorted, func(i, j int) bool {
		for _, opts := range sorts {
			ai := aggregate.New(
				sorted[i].AggregateName(),
				sorted[i].AggregateID(),
				aggregate.Version(sorted[i].AggregateVersion()),
			)
			aj := aggregate.New(
				sorted[j].AggregateName(),
				sorted[j].AggregateID(),
				aggregate.Version(sorted[j].AggregateVersion()),
			)
			cmp := opts.Sort.Compare(ai, aj)
			if cmp != 0 {
				return opts.Dir.Bool(cmp < 0)
			}
		}
		return true
	})

	return sorted
}

func (s *snapshot) Time() time.Time {
	return s.time
}

func (s *snapshot) Data() []byte {
	return s.data
}
