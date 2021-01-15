// Package nats provides an event.Bus implementation using a NATS client for transport.
package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
)

type eventBus struct {
	enc  event.Encoder
	once sync.Once
	conn *nats.Conn
}

type subscriber struct {
	msgs chan *nats.Msg
	sub  *nats.Subscription
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

// New returns an event.Bus that encodes and decodes event.Data using the
// provided Encoder. New() panics if enc is nil.
func New(enc event.Encoder) event.Bus {
	if enc == nil {
		panic("missing encoder")
	}
	return &eventBus{enc: enc}
}

func (bus *eventBus) Publish(ctx context.Context, events ...event.Event) error {
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

func (bus *eventBus) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, error) {
	if err := bus.connectOnce(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	subs := make([]subscriber, len(names))
	go bus.handleUnsubscribe(ctx, subs...)

	for i, name := range names {
		msgs := make(chan *nats.Msg)
		s, err := bus.conn.ChanSubscribe(name, msgs)
		if err != nil {
			panic(err)
			// TODO: handle err
		}
		sub := subscriber{
			msgs: msgs,
			sub:  s,
			done: make(chan struct{}),
		}
		subs[i] = sub
	}

	return bus.fanIn(subs...), nil
}

func (bus *eventBus) connectOnce(ctx context.Context) error {
	var err error
	bus.once.Do(func() { err = bus.connect(ctx) })
	return err
}

func (bus *eventBus) connect(ctx context.Context) error {
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

func (bus *eventBus) publish(ctx context.Context, evt event.Event) error {
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

func (bus *eventBus) fanIn(subs ...subscriber) <-chan event.Event {
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
					// TODO: handle err
					panic(err)
				}

				data, err := bus.enc.Decode(env.Name, bytes.NewReader(env.Data))
				if err != nil {
					// TODO: handle err
					panic(err)
				}

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

func (bus *eventBus) handleUnsubscribe(ctx context.Context, subs ...subscriber) {
	<-ctx.Done()
	log.Println(ctx.Err())
	for _, sub := range subs {
		if err := sub.sub.Unsubscribe(); err != nil {
			// TODO: handle err
			panic(err)
		}
		close(sub.done)
	}
}
