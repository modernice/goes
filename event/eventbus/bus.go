package eventbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modernice/goes/event"
)

type chanbus struct {
	sync.RWMutex

	artificialDelay time.Duration

	events map[string]*eventSubscription
	queue  chan event.Event
	done   chan struct{}
}

type eventSubscription struct {
	bus        *chanbus
	recipients []recipient

	subscribeQueue   chan subscribeJob
	unsubscribeQueue chan subscribeJob
	events           chan event.Event

	done chan struct{}
}

type recipient struct {
	events   chan event.Event
	errs     chan error
	unsubbed chan struct{}
}

type subscribeJob struct {
	rcpt recipient
	done chan struct{}
}

// Option is a type of function that modifies the properties of a [*chanbus]
// during its initialization. It enables the configuration of various settings
// of the event bus, such as the artificial delay, before it starts operation.
// Options are applied in the order they are provided when creating a new event
// bus through the New function.
type Option func(*chanbus)

// WithArtificialDelay sets an artificial delay for the event bus. This delay is
// applied after each event is published to the bus, effectively slowing down
// the rate of event publishing. The delay duration is specified by the provided
// time.Duration value. The function returns an Option that can be used to
// configure a chanbus instance.
func WithArtificialDelay(delay time.Duration) func(*chanbus) {
	return func(c *chanbus) {
		c.artificialDelay = delay
	}
}

// New creates a new instance of an event bus with the provided options. The
// returned event bus is safe for concurrent use and starts processing events
// immediately. The artificial delay parameter can be set to simulate network
// latency or other delays.
func New(opts ...Option) event.Bus {
	bus := &chanbus{
		artificialDelay: time.Millisecond,
		events:          make(map[string]*eventSubscription),
		queue:           make(chan event.Event),
		done:            make(chan struct{}),
	}
	for _, opt := range opts {
		opt(bus)
	}
	go bus.work()
	return bus
}

// Close signals the termination of the Chanbus. After Close is called, no more
// events can be processed by the Chanbus. Calling Close multiple times has no
// additional effect.
func (bus *chanbus) Close() {
	select {
	case <-bus.done:
	default:
		close(bus.done)
	}
}

// Subscribe is used to register interest in a set of events. It takes in a
// context and a variadic parameter of event names. For each event name
// provided, it creates a subscription that listens for these events. The
// function returns two channels: one for receiving the subscribed events and
// another for receiving any errors that may occur during the subscription
// process. If an error occurs while setting up any of the subscriptions, the
// function cancels all other subscriptions, closes the channels, and returns an
// error.
func (bus *chanbus) Subscribe(ctx context.Context, events ...string) (<-chan event.Event, <-chan error, error) {
	ctx, unsubscribeAll := context.WithCancel(ctx)
	go func() {
		<-bus.done
		unsubscribeAll()
	}()

	var rcpts []recipient

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

// Publish publishes the provided events to the event bus. It ensures each event
// is dispatched to all subscribed recipients. If an artificial delay is set, it
// pauses for that duration before dispatching the next event. The function
// returns an error if the context gets cancelled before all events are
// published.
func (bus *chanbus) Publish(ctx context.Context, events ...event.Event) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, evt := range events {
			select {
			case <-ctx.Done():
				return
			case bus.queue <- evt:
				if bus.artificialDelay > 0 {
					time.Sleep(bus.artificialDelay)
				}
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

func (bus *chanbus) subscribe(ctx context.Context, name string) (recipient, error) {
	bus.Lock()
	defer bus.Unlock()

	if sub, ok := bus.events[name]; ok {
		rcpt, err := sub.subscribe(ctx)
		if err != nil {
			return rcpt, fmt.Errorf("add recipient: %w [event=%v]", err, name)
		}
		return rcpt, nil
	}

	sub := &eventSubscription{
		bus:              bus,
		subscribeQueue:   make(chan subscribeJob),
		unsubscribeQueue: make(chan subscribeJob),
		events:           make(chan event.Event),
		done:             make(chan struct{}),
	}
	bus.events[name] = sub

	go sub.work()

	rcpt, err := sub.subscribe(ctx)
	if err != nil {
		return recipient{}, fmt.Errorf("add recipient: %w [event=%v]", err, name)
	}

	return rcpt, nil
}

func (bus *chanbus) work() {
	for evt := range bus.queue {
		bus.publish(evt)
	}
}

func (bus *chanbus) publish(evt event.Event) {
	bus.publishTo(evt.Name(), evt)
	bus.publishTo("*", evt)
}

func (bus *chanbus) publishTo(name string, evt event.Event) {
	bus.RLock()
	defer bus.RUnlock()
	if sub, ok := bus.events[name]; ok {
		sub.events <- evt
	}
}

func (sub *eventSubscription) subscribe(ctx context.Context) (recipient, error) {
	rcpt := recipient{
		events:   make(chan event.Event),
		errs:     make(chan error),
		unsubbed: make(chan struct{}),
	}

	job := subscribeJob{
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
			job := subscribeJob{
				rcpt: rcpt,
				done: make(chan struct{}),
			}
			sub.unsubscribeQueue <- job
			<-job.done
		}()
	}()

	select {
	case <-ctx.Done():
		return recipient{}, ctx.Err()
	case <-job.done:
		return rcpt, nil
	}
}

func (sub *eventSubscription) work() {
	for {
		select {
		case job := <-sub.subscribeQueue:
			sub.recipients = append(sub.recipients, job.rcpt)
			close(job.done)
		case job := <-sub.unsubscribeQueue:
			for i, rcpt := range sub.recipients {
				if rcpt == job.rcpt {
					close(rcpt.errs)
					close(rcpt.events)
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

func fanInEvents(ctx context.Context, rcpts []recipient) <-chan event.Event {
	out := make(chan event.Event)
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

func fanInErrors(ctx context.Context, rcpts []recipient) <-chan error {
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
