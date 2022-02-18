package eventbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
)

type chanbus[ID goes.ID] struct {
	sync.RWMutex

	events map[string]*eventSubscription[ID]
	queue  chan event.Of[any, ID]
	done   chan struct{}
}

type eventSubscription[ID goes.ID] struct {
	bus        *chanbus[ID]
	recipients []recipient[ID]

	subscribeQueue   chan subscribeJob[ID]
	unsubscribeQueue chan subscribeJob[ID]
	events           chan event.Of[any, ID]

	done chan struct{}
}

type recipient[ID goes.ID] struct {
	events   chan event.Of[any, ID]
	errs     chan error
	unsubbed chan struct{}
}

type subscribeJob[ID goes.ID] struct {
	rcpt recipient[ID]
	done chan struct{}
}

func New[ID goes.ID]() event.Bus[ID] {
	bus := &chanbus[ID]{
		events: make(map[string]*eventSubscription[ID]),
		queue:  make(chan event.Of[any, ID]),
	}
	go bus.work()
	return bus
}

func (bus *chanbus[ID]) Subscribe(ctx context.Context, events ...string) (<-chan event.Of[any, ID], <-chan error, error) {
	ctx, unsubscribeAll := context.WithCancel(ctx)
	go func() {
		// Will never happen, but makes the linter happy.
		<-bus.done
		unsubscribeAll()
	}()

	var rcpts []recipient[ID]

	for _, name := range events {
		rcpt, err := bus.subscribe(ctx, name)
		if err != nil {
			unsubscribeAll()
			return nil, nil, err
		}
		rcpts = append(rcpts, rcpt)
	}

	return fanInEvents(ctx, rcpts), fanInErrors(ctx, rcpts), nil
}

func (bus *chanbus[ID]) Publish(ctx context.Context, events ...event.Of[any, ID]) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, evt := range events {
			select {
			case <-ctx.Done():
				return
			case bus.queue <- evt:
			}
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (bus *chanbus[ID]) subscribe(ctx context.Context, name string) (recipient[ID], error) {
	bus.Lock()
	defer bus.Unlock()

	if sub, ok := bus.events[name]; ok {
		rcpt, err := sub.subscribe(ctx)
		if err != nil {
			return rcpt, fmt.Errorf("add recipient: %w [event=%v]", err, name)
		}
		return rcpt, nil
	}

	sub := &eventSubscription[ID]{
		bus:              bus,
		subscribeQueue:   make(chan subscribeJob[ID]),
		unsubscribeQueue: make(chan subscribeJob[ID]),
		events:           make(chan event.Of[any, ID]),
		done:             make(chan struct{}),
	}
	bus.events[name] = sub

	go sub.work()

	rcpt, err := sub.subscribe(ctx)
	if err != nil {
		return recipient[ID]{}, fmt.Errorf("add recipient: %w [event=%v]", err, name)
	}

	return rcpt, nil
}

func (bus *chanbus[ID]) work() {
	for evt := range bus.queue {
		bus.RLock()
		sub, ok := bus.events[evt.Name()]
		bus.RUnlock()
		if !ok {
			continue
		}
		sub.events <- evt
	}
}

func (sub *eventSubscription[ID]) subscribe(ctx context.Context) (recipient[ID], error) {
	rcpt := recipient[ID]{
		events:   make(chan event.Of[any, ID]),
		errs:     make(chan error),
		unsubbed: make(chan struct{}),
	}

	job := subscribeJob[ID]{
		rcpt: rcpt,
		done: make(chan struct{}),
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case sub.subscribeQueue <- job:
		}

		go func() {
			<-ctx.Done()
			close(rcpt.unsubbed)
			sub.unsubscribeQueue <- subscribeJob[ID]{
				rcpt: rcpt,
				done: make(chan struct{}),
			}
		}()
	}()

	select {
	case <-ctx.Done():
		return recipient[ID]{}, ctx.Err()
	case <-job.done:
		return rcpt, nil
	}
}

func (sub *eventSubscription[ID]) work() {
	for {
		select {
		case job := <-sub.subscribeQueue:
			sub.recipients = append(sub.recipients, job.rcpt)
			close(job.done)
		case job := <-sub.unsubscribeQueue:
			close(job.rcpt.events)
			close(job.rcpt.errs)
			for i, rcpt := range sub.recipients {
				if rcpt == job.rcpt {
					sub.recipients = append(sub.recipients[:i], sub.recipients[i+1:]...)
					break
				}
			}
			close(job.done)
		case evt := <-sub.events:
			for _, rcpt := range sub.recipients {
				select {
				case <-rcpt.unsubbed:
				case rcpt.events <- evt:
				}
			}
		}
	}
}

func fanInEvents[ID goes.ID](ctx context.Context, rcpts []recipient[ID]) <-chan event.Of[any, ID] {
	out := make(chan event.Of[any, ID])
	var wg sync.WaitGroup
	wg.Add(len(rcpts))
	go func() {
		wg.Wait()
		close(out)
	}()
	for _, rcpt := range rcpts {
		rcpt := rcpt
		go func() {
			defer wg.Done()
			for evt := range rcpt.events {
				select {
				case <-ctx.Done():
					return
				case out <- evt:
				}
			}
		}()
	}
	return out
}

func fanInErrors[ID goes.ID](ctx context.Context, rcpts []recipient[ID]) <-chan error {
	out := make(chan error)
	var wg sync.WaitGroup
	wg.Add(len(rcpts))
	go func() {
		wg.Wait()
		close(out)
	}()
	for _, rcpt := range rcpts {
		rcpt := rcpt
		go func() {
			defer wg.Done()
			for err := range rcpt.errs {
				select {
				case <-ctx.Done():
					return
				case out <- err:
				}
			}
		}()
	}
	return out
}
