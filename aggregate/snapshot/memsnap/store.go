package memsnap

import (
	"context"
	"errors"
	"sync"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/query"
)

var (
	// ErrNotFound is returned when trying to fetch a Snapshot that doesn't
	// exist.
	ErrNotFound = errors.New("snapshot not found")
)

type store[ID goes.ID] struct {
	sync.Mutex

	snaps map[string]map[ID]map[int]snapshot.Snapshot[ID]
}

// New returns an in-memory Snapshot Store.
func New[ID goes.ID]() snapshot.Store[ID] {
	return &store[ID]{
		snaps: make(map[string]map[ID]map[int]snapshot.Snapshot[ID]),
	}
}

func (s *store[ID]) Save(_ context.Context, snap snapshot.Snapshot[ID]) error {
	snaps := s.get(snap.AggregateName(), snap.AggregateID())
	s.Lock()
	defer s.Unlock()
	snaps[snap.AggregateVersion()] = snap
	return nil
}

func (s *store[ID]) Latest(_ context.Context, name string, id ID) (snapshot.Snapshot[ID], error) {
	snaps := s.get(name, id)
	if len(snaps) == 0 {
		return nil, ErrNotFound
	}
	var (
		v    int
		snap snapshot.Snapshot[ID]
	)
	for version, sn := range snaps {
		if version >= v {
			v = version
			snap = sn
		}
	}
	return snap, nil
}

func (s *store[ID]) Version(_ context.Context, name string, id ID, v int) (snapshot.Snapshot[ID], error) {
	snaps := s.get(name, id)
	snap, ok := snaps[v]
	if !ok {
		return nil, ErrNotFound
	}
	return snap, nil
}

func (s *store[ID]) Limit(_ context.Context, name string, id ID, v int) (snapshot.Snapshot[ID], error) {
	snaps := s.get(name, id)
	if len(snaps) == 0 {
		return nil, ErrNotFound
	}
	var (
		foundV int
		snap   snapshot.Snapshot[ID]
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

func (s *store[ID]) Query(ctx context.Context, q snapshot.Query[ID]) (<-chan snapshot.Snapshot[ID], <-chan error, error) {
	var snaps []snapshot.Snapshot[ID]
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

	out, outErrs := make(chan snapshot.Snapshot[ID]), make(chan error)

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

func (s *store[ID]) Delete(_ context.Context, snap snapshot.Snapshot[ID]) error {
	snaps := s.get(snap.AggregateName(), snap.AggregateID())
	s.Lock()
	defer s.Unlock()
	delete(snaps, snap.AggregateVersion())
	return nil
}

func (s *store[ID]) get(name string, id ID) map[int]snapshot.Snapshot[ID] {
	s.Lock()
	defer s.Unlock()
	snaps, ok := s.snaps[name]
	if !ok {
		snaps = make(map[ID]map[int]snapshot.Snapshot[ID])
		s.snaps[name] = snaps
	}
	isnaps, ok := snaps[id]
	if !ok {
		isnaps = make(map[int]snapshot.Snapshot[ID])
		snaps[id] = isnaps
	}
	return isnaps
}
