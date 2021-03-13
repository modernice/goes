package memsnap

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/query"
)

var (
	// ErrNotFound is returned when trying to fetch a Snapshot that doesn't
	// exist.
	ErrNotFound = errors.New("snapshot not found")
)

type store struct {
	sync.Mutex

	snaps map[string]map[uuid.UUID]map[int]snapshot.Snapshot
}

// New returns an in-memory Snapshot Store.
func New() snapshot.Store {
	return &store{
		snaps: make(map[string]map[uuid.UUID]map[int]snapshot.Snapshot),
	}
}

func (s *store) Save(_ context.Context, snap snapshot.Snapshot) error {
	snaps := s.get(snap.AggregateName(), snap.AggregateID())
	s.Lock()
	defer s.Unlock()
	snaps[snap.AggregateVersion()] = snap
	return nil
}

func (s *store) Latest(_ context.Context, name string, id uuid.UUID) (snapshot.Snapshot, error) {
	snaps := s.get(name, id)
	if len(snaps) == 0 {
		return nil, ErrNotFound
	}
	var (
		v    int
		snap snapshot.Snapshot
	)
	for version, sn := range snaps {
		if version >= v {
			v = version
			snap = sn
		}
	}
	return snap, nil
}

func (s *store) Version(_ context.Context, name string, id uuid.UUID, v int) (snapshot.Snapshot, error) {
	snaps := s.get(name, id)
	snap, ok := snaps[v]
	if !ok {
		return nil, ErrNotFound
	}
	return snap, nil
}

func (s *store) Limit(_ context.Context, name string, id uuid.UUID, v int) (snapshot.Snapshot, error) {
	snaps := s.get(name, id)
	if len(snaps) == 0 {
		return nil, ErrNotFound
	}
	var (
		foundV int
		snap   snapshot.Snapshot
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

func (s *store) Query(ctx context.Context, q snapshot.Query) (<-chan snapshot.Snapshot, <-chan error, error) {
	var snaps []snapshot.Snapshot
	for _, idsnaps := range s.snaps {
		for _, vsnaps := range idsnaps {
			for _, snap := range vsnaps {
				if !query.Test(q, snap) {
					continue
				}
				snaps = append(snaps, snap)
			}
		}
	}
	snaps = snapshot.SortMulti(snaps, q.Sortings()...)

	out, outErrs := make(chan snapshot.Snapshot), make(chan error)

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

func (s *store) Delete(_ context.Context, snap snapshot.Snapshot) error {
	snaps := s.get(snap.AggregateName(), snap.AggregateID())
	s.Lock()
	defer s.Unlock()
	delete(snaps, snap.AggregateVersion())
	return nil
}

func (s *store) get(name string, id uuid.UUID) map[int]snapshot.Snapshot {
	s.Lock()
	defer s.Unlock()
	snaps, ok := s.snaps[name]
	if !ok {
		snaps = make(map[uuid.UUID]map[int]snapshot.Snapshot)
		s.snaps[name] = snaps
	}
	isnaps, ok := snaps[id]
	if !ok {
		isnaps = make(map[int]snapshot.Snapshot)
		snaps[id] = isnaps
	}
	return isnaps
}
