package snapshot

import (
	"context"
	"errors"
	"sync"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event/query/time"
)

var (
	// ErrNotFound is returned when trying to fetch a Snapshot that doesn't
	// exist.
	ErrNotFound = errors.New("snapshot not found")
)

type store struct {
	sync.Mutex

	snaps map[string]map[uuid.UUID]map[int]Snapshot
}

// NewStore returns an in-memory Snapshot Store.
func NewStore() Store {
	return &store{
		snaps: make(map[string]map[uuid.UUID]map[int]Snapshot),
	}
}

// Save saves the given Snapshot to an in-memory store. It takes a
// context.Context as its first argument and a snapshot.Snapshot as its second
// argument. It returns an error if the operation fails.
func (s *store) Save(_ context.Context, snap Snapshot) error {
	snaps := s.get(snap.AggregateName(), snap.AggregateID())
	s.Lock()
	defer s.Unlock()
	snaps[snap.AggregateVersion()] = snap
	return nil
}

// Latest retrieves the latest Snapshot of an Aggregate with a given name and ID
// from an in-memory Snapshot Store [store.Store]. If no Snapshots for the given
// Aggregate exist, ErrNotFound is returned.
func (s *store) Latest(_ context.Context, name string, id uuid.UUID) (Snapshot, error) {
	snaps := s.get(name, id)
	if len(snaps) == 0 {
		return nil, ErrNotFound
	}
	var (
		v    int
		snap Snapshot
	)
	for version, sn := range snaps {
		if version >= v {
			v = version
			snap = sn
		}
	}
	return snap, nil
}

// Version retrieves the Snapshot of an Aggregate with a specific version number
// [Snapshot, AggregateName, AggregateID]. If the Snapshot doesn't exist,
// ErrNotFound is returned.
func (s *store) Version(_ context.Context, name string, id uuid.UUID, v int) (Snapshot, error) {
	snaps := s.get(name, id)
	snap, ok := snaps[v]
	if !ok {
		return nil, ErrNotFound
	}
	return snap, nil
}

// Limit returns a Snapshot with the version number less than or equal to the
// given version number (v) for the specified aggregate name and ID. If no such
// Snapshot is found, it returns an ErrNotFound error.
func (s *store) Limit(_ context.Context, name string, id uuid.UUID, v int) (Snapshot, error) {
	snaps := s.get(name, id)
	if len(snaps) == 0 {
		return nil, ErrNotFound
	}
	var (
		foundV int
		snap   Snapshot
	)
	for version, sn := range snaps {
		if version >= foundV && version <= v {
			foundV = version
			snap = sn
		}
	}
	if snap == nil {
		return nil, ErrNotFound
	}
	return snap, nil
}

// Query returns a channel of Snapshots and a channel of errors. It takes a
// context.Context and a Query as arguments. The channel of Snapshots will
// contain all the Snapshots that satisfy the Query. The channel of errors will
// contain any errors that occur during the Query.
func (s *store) Query(ctx context.Context, q Query) (<-chan Snapshot, <-chan error, error) {
	var snaps []Snapshot
	for _, idsnaps := range s.snaps {
		for _, vsnaps := range idsnaps {
			for _, snap := range vsnaps {
				if !Test(q, snap) {
					continue
				}
				snaps = append(snaps, snap)
			}
		}
	}
	snaps = SortMulti(snaps, q.Sortings()...)

	out, outErrs := make(chan Snapshot), make(chan error)

	go func() {
		defer close(out)
		defer close(outErrs)
		for _, snap := range snaps {
			select {
			case <-ctx.Done():
				return
			case out <- snap:
			}
		}
	}()

	return out, outErrs, nil
}

// Delete removes a Snapshot from the in-memory store.
func (s *store) Delete(_ context.Context, snap Snapshot) error {
	snaps := s.get(snap.AggregateName(), snap.AggregateID())
	s.Lock()
	defer s.Unlock()
	delete(snaps, snap.AggregateVersion())
	return nil
}

func (s *store) get(name string, id uuid.UUID) map[int]Snapshot {
	s.Lock()
	defer s.Unlock()
	snaps, ok := s.snaps[name]
	if !ok {
		snaps = make(map[uuid.UUID]map[int]Snapshot)
		s.snaps[name] = snaps
	}
	isnaps, ok := snaps[id]
	if !ok {
		isnaps = make(map[int]Snapshot)
		snaps[id] = isnaps
	}
	return isnaps
}

func timesContains(times []stdtime.Time, t stdtime.Time) bool {
	for _, v := range times {
		if v.Equal(t) {
			return true
		}
	}
	return false
}

func testTimeRanges(ranges []time.Range, t stdtime.Time) bool {
	for _, r := range ranges {
		if r.Includes(t) {
			return true
		}
	}
	return false
}

func testMinTimes(min stdtime.Time, t stdtime.Time) bool {
	if t.Equal(min) || t.After(min) {
		return true
	}
	return false
}

func testMaxTimes(max stdtime.Time, t stdtime.Time) bool {
	if t.Equal(max) || t.Before(max) {
		return true
	}
	return false
}
