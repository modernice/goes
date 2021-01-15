// Package nats provides an event.Bus implementation using a NATS client for transport.
package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
)

// EventBus is the NATS event.Bus implementation.
type EventBus struct {
	enc  event.Encoder
	conn *nats.Conn

	errs    chan *Error
	errMux  sync.RWMutex
	errSubs []errorSubscriber

	onceConnect sync.Once
	onceErrors  sync.Once
}

type subscriber struct {
	msgs chan *nats.Msg
	sub  *nats.Subscription
	done chan struct{}
}

type errorSubscriber struct {
	ctx  context.Context
	errs chan *Error
	done chan struct{}
}

type envelope struct {
	ID               uuid.UUID
	Name             string
	Time             time.Time
	Data             []byte
	AggregateName    string
	AggregateID      uuid.UUID
	AggregateVersion int
}

// New returns a new EventBus that encodes and decodes event.Data using the
// provided Encoder. New panics if enc is nil.
func New(enc event.Encoder) *EventBus {
	if enc == nil {
		panic("missing encoder")
	}
	return &EventBus{
		enc:  enc,
		errs: make(chan *Error),
	}
}

// Publish sends each Event evt in events to subscribers who
// subscribed to Events with a name of evt.Name().
func (bus *EventBus) Publish(ctx context.Context, events ...event.Event) error {
	if err := bus.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	for _, evt := range events {
		if err := bus.publish(ctx, evt); err != nil {
			return fmt.Errorf(`publish "%s" event: %w`, evt.Name(), err)
		}
	}

	return nil
}

func (bus *EventBus) publish(ctx context.Context, evt event.Event) error {
	var buf bytes.Buffer
	if err := bus.enc.Encode(&buf, evt.Data()); err != nil {
		return fmt.Errorf("encode event data: %w", err)
	}

	env := envelope{
		ID:               evt.ID(),
		Name:             evt.Name(),
		Time:             evt.Time(),
		Data:             buf.Bytes(),
		AggregateName:    evt.AggregateName(),
		AggregateID:      evt.AggregateID(),
		AggregateVersion: evt.AggregateVersion(),
	}

	buf.Reset()
	if err := gob.NewEncoder(&buf).Encode(env); err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}

	if err := bus.conn.Publish(evt.Name(), buf.Bytes()); err != nil {
		return fmt.Errorf("nats: %w", err)
	}

	return nil
}

// Subscribe returns a channel of Events. For every published Event evt
// where evt.Name() is one of names, that Event will be received from the
// returned Event channel. When ctx is canceled, events will be closed.
func (bus *EventBus) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, error) {
	if err := bus.connectOnce(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	subs := make([]subscriber, 0, len(names))

	var subscribeError error
	for _, name := range names {
		msgs := make(chan *nats.Msg)
		s, err := bus.conn.ChanSubscribe(name, msgs)
		if err != nil {
			subscribeError = err
			break
		}
		sub := subscriber{
			msgs: msgs,
			sub:  s,
			done: make(chan struct{}),
		}
		subs = append(subs, sub)
	}

	// if subscription failed for an event name, cancel ctx immediately and let
	// bus.handleUnsubscribe handle the cleanup
	var cancel context.CancelFunc
	if subscribeError != nil {
		ctx, cancel = context.WithCancel(ctx)
		cancel()
	}

	go bus.handleUnsubscribe(ctx, subs...)

	return bus.fanIn(subs...), nil
}

func (bus *EventBus) connectOnce(ctx context.Context) error {
	var err error
	bus.onceConnect.Do(func() { err = bus.connect(ctx) })
	return err
}

func (bus *EventBus) connect(ctx context.Context) error {
	connectError := make(chan error)
	go func() {
		var err error
		uri := nats.DefaultURL
		if envuri := os.Getenv("NATS_URI"); envuri != "" {
			uri = envuri
		}
		if bus.conn, err = nats.Connect(uri); err != nil {
			connectError <- fmt.Errorf("nats: %w", err)
			return
		}
		connectError <- nil
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-connectError:
		return err
	}
}

func (bus *EventBus) fanIn(subs ...subscriber) <-chan event.Event {
	events := make(chan event.Event)

	var wg sync.WaitGroup
	wg.Add(len(subs))

	// close events channel when all subscribers done
	go func() {
		wg.Wait()
		close(events)
	}()

	// for every subscriber sub wait until sub.done is closed, then decrement
	// wait counter
	for _, sub := range subs {
		go func(sub subscriber) {
			defer wg.Done()
			<-sub.done
		}(sub)
	}

	for _, sub := range subs {
		go func(sub subscriber) {
			for msg := range sub.msgs {
				var env envelope
				if err := gob.NewDecoder(bytes.NewReader(msg.Data)).Decode(&env); err != nil {
					bus.error(fmt.Errorf("gob decode envelope: %w", err), nil)
					continue
				}

				data, err := bus.enc.Decode(env.Name, bytes.NewReader(env.Data))
				if err != nil {
					bus.error(fmt.Errorf(`encode "%s" event data: %w`, env.Name, err), nil)
					continue
				}

				// TODO: timeout
				events <- event.New(
					env.Name,
					data,
					event.ID(env.ID),
					event.Time(env.Time),
					event.Aggregate(env.AggregateName, env.AggregateID, env.AggregateVersion),
				)
			}
		}(sub)
	}
	return events
}

func (bus *EventBus) handleUnsubscribe(ctx context.Context, subs ...subscriber) {
	<-ctx.Done()
	for _, sub := range subs {
		if err := sub.sub.Unsubscribe(); err != nil {
			bus.error(fmt.Errorf(`unsubscribe from subject "%s": %w`, sub.sub.Subject, err), nil)
		}
		close(sub.done)
	}
}

// Errors returns an Error channel that receives future asynchronous errors from
// the EventBus. When ctx is canceled, the Error channel wil be closed.
func (bus *EventBus) Errors(ctx context.Context) <-chan *Error {
	// start sending errors to subscribers on first subscription
	bus.onceErrors.Do(bus.goHandleErrors)

	errs := make(chan *Error)
	done := make(chan struct{})
	sub := errorSubscriber{
		ctx:  ctx,
		errs: errs,
		done: done,
	}

	bus.errMux.Lock()
	bus.errSubs = append(bus.errSubs, sub)
	bus.errMux.Unlock()

	go bus.handleErrorUnsubscribe(sub)

	return errs
}

func (bus *EventBus) goHandleErrors() {
	go bus.handleErrors()
}

func (bus *EventBus) handleErrors() {
	for err := range bus.errs {
		bus.errMux.RLock()
		for _, sub := range bus.errSubs {
			go func(sub errorSubscriber, err *Error) {
				select {
				// abort send of error to subscriber if its context is canceled
				// and/or its errors channel is full
				case <-sub.ctx.Done():
				case sub.errs <- err:
				}
			}(sub, err)
		}
		bus.errMux.RUnlock()
	}
}

func (bus *EventBus) handleErrorUnsubscribe(sub errorSubscriber) {
	defer close(sub.done)

	// close the subscription's error channel when done
	go func() {
		<-sub.done
		close(sub.errs)
	}()

	<-sub.ctx.Done()
	bus.errMux.Lock()
	defer bus.errMux.Unlock()

	for i, errSub := range bus.errSubs {
		if sub == errSub {
			bus.errSubs = append(bus.errSubs[:i], bus.errSubs[i+1:]...)
		}
	}
}

func (bus *EventBus) error(err error, evt event.Event) {
	bus.errs <- &Error{Err: err, Event: evt}
}
