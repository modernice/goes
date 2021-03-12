package memsnap

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/snapshot"
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

type stream struct {
	snaps []snapshot.Snapshot
	index int
	snap  snapshot.Snapshot
	err   error
}

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

func (s *store) Query(ctx context.Context, q aggregate.Query) (<-chan snapshot.Snapshot, <-chan error, error) {
	var snaps []snapshot.Snapshot
	for name, idsnaps := range s.snaps {
		for id, vsnaps := range idsnaps {
			for v, snap := range vsnaps {
				if !query.Test(q, aggregate.New(name, id, aggregate.Version(v))) {
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

func (s *stream) Next(context.Context) bool {
	if s.index < len(s.snaps) {
		s.snap = s.snaps[s.index]
		s.index++
		return true
	}
	return false
}

func (s *stream) Snapshot() snapshot.Snapshot {
	return s.snap
}

func (s *stream) Err() error {
	return nil
}

func (s *stream) Close(context.Context) error {
	return nil
}
