package memory

import (
	"context"
	"ecommerce/order"
	"sync"

	"github.com/google/uuid"
)

type timelines struct {
	sync.RWMutex

	store map[uuid.UUID]*order.Timeline
}

// NewTimelineRepository returns an in-memory TimelineRepository.
func NewTimelineRepository() order.TimelineRepository {
	return &timelines{
		store: make(map[uuid.UUID]*order.Timeline),
	}
}

func (r *timelines) Save(ctx context.Context, tl *order.Timeline) error {
	r.Lock()
	defer r.Unlock()
	r.store[tl.ID] = tl
	return nil
}

func (r *timelines) Fetch(ctx context.Context, id uuid.UUID) (*order.Timeline, error) {
	r.Lock()
	defer r.Unlock()
	if tl, ok := r.store[id]; ok {
		return tl, nil
	}
	return nil, order.ErrTimelineNotFound
}
