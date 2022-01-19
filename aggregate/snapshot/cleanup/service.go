package cleanup

import (
	"context"
	"errors"
	"fmt"
	stdtime "time"

	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/query"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/xtime"
)

var (
	// ErrStarted is returned when trying to start an already started Service.
	ErrStarted = errors.New("service started")

	// ErrStopped is returned when trying to stop a Service that is not started.
	ErrStopped = errors.New("service stopped")
)

var (
	errNilStore = errors.New("nil Store")
)

// Service is a Snapshot cleanup service which deletes Snapshots that exceed a
// given age.
type Service struct {
	every  stdtime.Duration
	maxAge stdtime.Duration

	ctx    context.Context
	cancel context.CancelFunc

	started bool
	done    chan struct{}
}

// NewService returns a new Service which periodically deletes old Snapshots
// every `every` Duration. Snapshots that exceed the given maxAge will be
// deleted by the Service.
//
// Example:
//
//	// Every day, delete Snapshots that are older than a week.
//	var store snapshot.Store
//	svc := cleanup.NewService(24*time.Hour, 7*24*time.Hour)
//	errs, err := svc.Start(store)
//	// handle err
//	for err := range errs {
//		// handle async err
//	}
func NewService(every, maxAge stdtime.Duration) *Service {
	return &Service{
		every:  every,
		maxAge: maxAge,
	}
}

// Start starts the cleanup service and returns a channel of asynchronous errors
// that occur during a cleanup. Callers of Start must receive from the returned
// error channel; otherwise the Service will block when an error occurs. The
// returned error channel is closed when the Service is stopped.
//
// Start returns a nil channel and ErrStarted if the Service has been started
// already.
//
// Example:
//
//	var store snapshot.Store
// 	svc := cleanup.NewService(24*time.Hour, 7*24*time.Hour)
//	errs, err := svc.Start(store)
//	// handle err
//	go func() {
//		for err := range errs {
//			// handle async err
//		}
//	}()
func (c *Service) Start(store snapshot.Store) (<-chan error, error) {
	if store == nil {
		return nil, errNilStore
	}

	if c.started {
		return nil, ErrStarted
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())
	out := make(chan error)
	c.done = make(chan struct{})
	c.started = true

	go c.work(store, out)

	return out, nil
}

func (c *Service) work(store snapshot.Store, out chan<- error) {
	defer close(c.done)
	defer close(out)
	ticker := stdtime.NewTicker(c.every)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.cleanup(store, out)
		}
	}
}

func (c *Service) cleanup(store snapshot.Store, out chan<- error) error {
	str, errs, err := store.Query(c.ctx, query.New(
		query.Time(time.Before(xtime.Now().Add(-c.maxAge))),
	))
	if err != nil {
		return fmt.Errorf("query Snapshots: %w", err)
	}
	return streams.Walk(c.ctx, func(s snapshot.Snapshot) error {
		if err := store.Delete(c.ctx, s); err != nil {
			select {
			case <-c.ctx.Done():
				return c.ctx.Err()
			case out <- fmt.Errorf("delete Snapshot: %w", err):
			}
		}
		return nil
	}, str, errs)
}

// Stop stops the Service. Should ctx be canceled before Stop completes,
// ctx.Err() is returned. If the Service has already been started, Stop returns
// ErrStopped.
//
// Example:
//
//	svc := cleanup.NewService(24*time.Hour, 7*24*time.Hour)
//	errs, err := svc.Start()
//	// handle err & async errs
//	if err = svc.Stop(context.TODO()); err != nil {
//		// handle err
//	}
func (c *Service) Stop(ctx context.Context) error {
	if !c.started {
		return ErrStopped
	}
	c.cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		c.started = false
		return nil
	}
}
