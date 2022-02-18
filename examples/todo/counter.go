package todo

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

// Counter provides the number of active, removed and archived tasks.
type Counter struct {
	*projection.Base[uuid.UUID]

	sync.RWMutex
	active   int
	removed  int
	archived int
}

// NewCounter returns a new task counter.
func NewCounter() *Counter {
	c := &Counter{Base: projection.New[uuid.UUID]()}

	// Register event appliers for each of the projection events.
	projection.ApplyWith[string, uuid.UUID](c, TaskAdded, c.taskAdded)
	projection.ApplyWith[TaskRemovedEvent, uuid.UUID](c, TaskRemoved, c.taskRemoved)
	projection.ApplyWith[[]string, uuid.UUID](c, TasksDone, c.tasksDone)

	return c
}

// Active returns the active tasks.
func (c *Counter) Active() int {
	c.RLock()
	defer c.RUnlock()
	return c.active
}

// Removed returns the removed tasks.
func (c *Counter) Removed() int {
	c.RLock()
	defer c.RUnlock()
	return c.removed
}

// Archived returns the archived tasks.
func (c *Counter) Archived() int {
	c.RLock()
	defer c.RUnlock()
	return c.archived
}

// Project projects the Counter until ctx is canceled. Each time one of
// TaskEvents is published, the counter is updated.
func (c *Counter) Project(
	ctx context.Context,
	bus event.Bus[uuid.UUID],
	store event.Store[uuid.UUID],
	opts ...schedule.ContinuousOption[uuid.UUID],
) (<-chan error, error) {
	s := schedule.Continuously(bus, store, TaskEvents[:], opts...)

	errs, err := s.Subscribe(ctx, func(ctx projection.Job[uuid.UUID]) error {
		log.Println("Applying job ...")

		c.Lock()
		defer c.Unlock()
		defer c.print()
		return ctx.Apply(ctx, c)
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe to projection schedule: %w", err)
	}

	return errs, nil
}

func (c *Counter) taskAdded(evt event.Of[string, uuid.UUID]) {
	c.active++
}

func (c *Counter) taskRemoved(evt event.Of[TaskRemovedEvent, uuid.UUID]) {
	c.removed++
	c.active--
}

func (c *Counter) tasksDone(evt event.Of[[]string, uuid.UUID]) {
	c.archived++
	c.active -= len(evt.Data())
}

func (c *Counter) print() {
	log.Printf("[Counter] Active: %d, Removed: %d, Archived: %d", c.active, c.removed, c.archived)
}
