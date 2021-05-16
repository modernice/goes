package memtags

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/tagging"
)

type store struct {
	sync.RWMutex

	tags       map[tagging.Aggregate][]string
	aggregates map[string]aggregates
}

type aggregates []tagging.Aggregate

// NewStore returns an in-memory tagging store.
func NewStore() tagging.Store {
	return &store{
		tags:       make(map[tagging.Aggregate][]string),
		aggregates: make(map[string]aggregates),
	}
}

func (s *store) Update(_ context.Context, name string, id uuid.UUID, tags []string) error {
	s.Lock()
	defer s.Unlock()

	a := tagging.Aggregate{
		Name: name,
		ID:   id,
	}

	oldTags := s.tags[a]
	s.tags[a] = tags

	for _, tag := range oldTags {
		as, ok := s.aggregates[tag]
		if !ok {
			as = make(aggregates, 0)
			s.aggregates[tag] = as
		}
		s.aggregates[tag] = as.remove(a)
	}

	for _, tag := range tags {
		as, ok := s.aggregates[tag]
		if !ok {
			as = make(aggregates, 0)
			s.aggregates[tag] = as
		}
		s.aggregates[tag] = as.add(a)
	}

	return nil
}

func (s *store) Tags(_ context.Context, name string, id uuid.UUID) ([]string, error) {
	s.RLock()
	defer s.RUnlock()
	return s.tags[tagging.Aggregate{Name: name, ID: id}], nil
}

func (s *store) TaggedWith(_ context.Context, tags ...string) ([]tagging.Aggregate, error) {
	s.RLock()
	defer s.RUnlock()

	var all []tagging.Aggregate

	if len(tags) > 0 {
		for _, tag := range tags {
			all = append(all, s.aggregates[tag]...)
		}
	} else {
		for _, as := range s.aggregates {
			all = append(all, as...)
		}
	}

	uniqmap := make(map[tagging.Aggregate]bool)
	out := make([]tagging.Aggregate, 0, len(all))
	for _, a := range all {
		if uniqmap[a] {
			continue
		}
		uniqmap[a] = true
		out = append(out, a)
	}
	return out, nil
}

func (as aggregates) add(a tagging.Aggregate) aggregates {
	for _, a2 := range as {
		if a2 == a {
			return as
		}
	}
	return append(as, a)
}

func (as aggregates) remove(a tagging.Aggregate) aggregates {
	for i, a2 := range as {
		if a2 == a {
			return append(as[:i], as[i+1:]...)
		}
	}
	return as
}
