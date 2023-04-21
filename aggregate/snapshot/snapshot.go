package snapshot

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/internal/xtime"
)

// Snapshot is a snapshot of an aggregate.
type Snapshot interface {
	// AggregateName returns the name of the aggregate.
	AggregateName() string

	// AggregateID returns the UUID of the aggregate.
	AggregateID() uuid.UUID

	// AggregateVersion returns the version of the aggregate at the time of the snapshot.
	AggregateVersion() int

	// Time returns the time of the snapshot.
	Time() time.Time

	// State returns the encoded state of the aggregate at the time of the snapshot.
	State() []byte
}

// Option is an option for creating a snapshot.
type Option func(*snapshot)

type snapshot struct {
	id      uuid.UUID
	name    string
	version int
	time    time.Time
	state   []byte
}

// Time returns an Option that sets the Time of a snapshot.
func Time(t time.Time) Option {
	return func(s *snapshot) {
		s.time = t
	}
}

// Data returns an Option that overrides the encoded data of a snapshot.
func Data(b []byte) Option {
	return func(s *snapshot) {
		s.state = b
	}
}

// New creates and returns a snapshot of the given aggregate.
func New(a aggregate.Aggregate, opts ...Option) (Snapshot, error) {
	id, name, v := a.Aggregate()

	snap := snapshot{
		id:      id,
		name:    name,
		version: v,
		time:    xtime.Now(),
	}
	for _, opt := range opts {
		opt(&snap)
	}

	if snap.state == nil {
		if b, err := Marshal(a); err == nil {
			snap.state = b
		} else if !errors.Is(err, ErrUnimplemented) {
			return snap, fmt.Errorf("marshal snapshot: %w", err)
		}
	}

	return &snap, nil
}

// AggregateID returns the UUID of a Snapshot's aggregate [uuid.UUID].
func (s snapshot) AggregateID() uuid.UUID {
	return s.id
}

// AggregateName returns the name of the aggregate associated with a Snapshot.
func (s snapshot) AggregateName() string {
	return s.name
}

// AggregateVersion returns the version of the aggregate at the time of the
// snapshot. It is a method of the Snapshot interface.
func (s snapshot) AggregateVersion() int {
	return s.version
}

// Time returns the time of the snapshot. It is a method of the Snapshot
// interface [snapshot.Snapshot].
func (s snapshot) Time() time.Time {
	return s.time
}

// State represents the encoded state of an aggregate at the time of a snapshot.
// It is returned by the State() method of a Snapshot.
func (s snapshot) State() []byte {
	return s.state
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
