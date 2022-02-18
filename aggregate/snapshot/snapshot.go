package snapshot

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/internal/xtime"
)

// Snapshot is a snapshot of an Aggregate.
type Snapshot[ID goes.ID] interface {
	// AggregateName returns the name of the aggregate.
	AggregateName() string

	// AggregateID returns the UUID of the aggregate.
	AggregateID() ID

	// AggregateVersion returns the version of the aggregate at the time of the snapshot.
	AggregateVersion() int

	// Time returns the time of the snapshot.
	Time() time.Time

	// State returns the encoded state of the aggregate at the time of the snapshot.
	State() []byte
}

// Option is an option for creating a snapshot.
type Option func(*options)

type options struct {
	time  time.Time
	state []byte
}

type snapshot[ID goes.ID] struct {
	options
	id      ID
	name    string
	version int
}

// Time returns an Option that sets the Time of a snapshot.
func Time(t time.Time) Option {
	return func(opts *options) {
		opts.time = t
	}
}

// Data returns an Option that overrides the encoded data of a snapshot.
func Data(b []byte) Option {
	return func(opts *options) {
		opts.state = b
	}
}

// New creates and returns a snapshot of the given aggregate.
func New[ID goes.ID](a aggregate.AggregateOf[ID], opts ...Option) (Snapshot[ID], error) {
	id, name, v := a.Aggregate()

	snap := snapshot[ID]{
		id:      id,
		name:    name,
		version: v,
		options: options{time: xtime.Now()},
	}
	for _, opt := range opts {
		opt(&snap.options)
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

func (s snapshot[ID]) AggregateID() ID {
	return s.id
}

func (s snapshot[ID]) AggregateName() string {
	return s.name
}

func (s snapshot[ID]) AggregateVersion() int {
	return s.version
}

func (s snapshot[ID]) Time() time.Time {
	return s.time
}

func (s snapshot[ID]) State() []byte {
	return s.state
}

// Sort sorts Snapshot and returns the sorted Snapshots.
func Sort[ID goes.ID](snaps []Snapshot[ID], s aggregate.Sorting, dir aggregate.SortDirection) []Snapshot[ID] {
	return SortMulti(snaps, aggregate.SortOptions{Sort: s, Dir: dir})
}

// SortMulti sorts Snapshots by multiple fields and returns the sorted
// aggregates.
func SortMulti[ID goes.ID](snaps []Snapshot[ID], sorts ...aggregate.SortOptions) []Snapshot[ID] {
	sorted := make([]Snapshot[ID], len(snaps))
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
			cmp := aggregate.CompareSorting[ID](opts.Sort, ai, aj)
			if cmp != 0 {
				return opts.Dir.Bool(cmp < 0)
			}
		}
		return true
	})

	return sorted
}
