// Package nats provides an event bus that uses NATS to publish and subscribe to
// events over a (distributed) network with support for both NATS Core and NATS
// JetStream.
package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/env"
	"github.com/nats-io/nats.go"
)

var (
	// ErrPullTimeout is raised by an EventBus when a subscriber doesn't pull an
	// event from the event channel within the specified PullTimeout. In such
	// case, the event is dropped to avoid blocking the application because of a
	// slow consumer.
	ErrPullTimeout = errors.New("pull timed out. slow consumer?")
)

// EventBus is an event bus that uses NATS to publish and subscribe to events.
//
// Drivers
//
// The event bus supports both NATS Core and NATS JetStream. By default, the
// Core driver is used, but you can create and specify the JetStream driver with
// the Use option:
//
//	var enc codec.Encoding
//	bus := nats.NewEventBus(enc, nats.Use(nats.JetStream()))
type EventBus struct {
	enc codec.Encoding

	eatErrors   bool
	url         string
	pullTimeout time.Duration

	subjectFunc    func(eventName string) (subject string)
	queueFunc      func(eventName string) (queue string)
	durableFunc    func(subject string, queue string) string
	streamNameFunc func(subject, queue string) string

	conn     *nats.Conn
	natsOpts []nats.Option
	subOpts  []nats.SubOpt
	driver   Driver

	onceConnect sync.Once
}

// EventBusOption is an option for an EventBus.
type EventBusOption func(*EventBus)

// A Driver provides the specific implementation for interacting with either
// NATS Core or NATS JetStream. Use the Core or JetStream functions to create
// a Driver.
type Driver interface {
	name() string
	subscribe(ctx context.Context, bus *EventBus, subject string) (*subscription, error)
	publish(ctx context.Context, bus *EventBus, evt event.Event) error
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

type subscription struct {
	sub       *nats.Subscription
	msgs      chan []byte
	errs      chan error
	unsubbbed chan struct{}
}

// NewEventBus returns a NATS event bus.
//
// The provided Encoder is used to encode and decode event data when publishing
// and subscribing to events.
//
// If no other specified, the returned event bus will use the NATS Core Driver.
// To use the NATS JetStream Driver instead, explicitly set the Driver:
//	NewEventBus(enc, Use(JetStream()))
func NewEventBus(enc codec.Encoding, opts ...EventBusOption) *EventBus {
	if enc == nil {
		enc = codec.New()
	}

	bus := &EventBus{enc: enc}
	for _, opt := range opts {
		opt(bus)
	}
	bus.init()

	return bus
}

// Connects connects to NATS.
//
// It is not required to call Connect to use the EventBus because Connect is
// automatically called by Subscribe and Publish.
func (bus *EventBus) Connect(ctx context.Context) error {
	var err error
	bus.onceConnect.Do(func() {
		if err = bus.connect(ctx); err != nil {
			return
		}

		// The JetStream driver initializes the JetStreamContext.
		if d, ok := bus.driver.(interface{ init(*EventBus) error }); ok {
			if err = d.init(bus); err != nil {
				return
			}
		}
	})
	return err
}

func (bus *EventBus) connect(ctx context.Context) error {
	// *nats.Conn provided via Conn() option.
	if bus.conn != nil {
		return nil
	}

	connectError := make(chan error)
	go func() {
		url := bus.natsURL()
		var err error
		if bus.conn, err = nats.Connect(url, bus.natsOpts...); err != nil {
			connectError <- fmt.Errorf("connect: %w [url=%v]", err, url)
			return
		}
		connectError <- nil
	}()

	return <-connectError
}

// Publish publishes events.
func (bus *EventBus) Publish(ctx context.Context, events ...event.Event) error {
	if err := bus.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	for _, evt := range events {
		if err := bus.driver.publish(ctx, bus, evt); err != nil {
			return fmt.Errorf("publish event: %w [event=%v]", err, evt.Name())
		}
	}

	return nil
}

// Subscribe subscribes to events.
func (bus *EventBus) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, <-chan error, error) {
	if err := bus.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}

	subs := make([]*subscription, len(names))
	for i, name := range names {
		sub, err := bus.driver.subscribe(ctx, bus, name)
		if err != nil {
			return nil, nil, fmt.Errorf("subscribe: %w [event=%v]", err, name)
		}
		subs[i] = sub
	}

	events, errs := bus.fanIn(ctx, subs)

	if bus.eatErrors {
		go discardErrors(errs)
	}

	return events, errs, nil
}

func (bus *EventBus) init(opts ...EventBusOption) error {
	var envOpts []EventBusOption
	if env.Bool("NATS_QUEUE_GROUP_BY_EVENT") {
		envOpts = append(envOpts, QueueGroupByEvent())
	}

	if service := env.String("NATS_LOAD_BALANCER"); service != "" {
		envOpts = append(envOpts, WithLoadBalancer(service))
	}

	if prefix := strings.TrimSpace(env.String("NATS_SUBJECT_PREFIX")); prefix != "" {
		envOpts = append(envOpts, SubjectPrefix(prefix))
	}

	if bus.durableFunc == nil {
		fn, err := envDurableNameFunc()
		if err != nil {
			panic(err)
		}
		bus.durableFunc = fn
	}

	if env.String("NATS_PULL_TIMEOUT") != "" {
		if d, err := env.Duration("NATS_PULL_TIMEOUT"); err == nil {
			envOpts = append(envOpts, PullTimeout(d))
		} else {
			panic(fmt.Errorf(
				"init: parse environment variable %q: %w",
				"NATS_PULL_TIMEOUT",
				err,
			))
		}
	}

	opts = append(envOpts, opts...)
	for _, opt := range opts {
		opt(bus)
	}

	if bus.queueFunc == nil {
		bus.queueFunc = noQueue
	}

	if bus.subjectFunc == nil {
		bus.subjectFunc = defaultSubjectFunc
	}

	// If the StreamNameFunc option is not used, use the DurableFunc option for
	// the stream names.
	//
	// Note: Only used by JetStream driver.
	if bus.streamNameFunc == nil {
		bus.streamNameFunc = bus.durableFunc
	}

	if bus.driver == nil {
		bus.driver = Core()
	}

	return nil
}

func (bus *EventBus) natsURL() string {
	if bus.url != "" {
		return bus.url
	}
	if url := os.Getenv("NATS_URL"); url != "" {
		return url
	}
	return nats.DefaultURL
}

// Fan-in events and errors from multiple subscriptions.
func (bus *EventBus) fanIn(ctx context.Context, subs []*subscription) (<-chan event.Event, <-chan error) {
	return bus.fanInEvents(ctx, subs), fanInErrors(subs)
}

// Fan-in events from multiple subscriptions.
func (bus *EventBus) fanInEvents(ctx context.Context, subs []*subscription) <-chan event.Event {
	out := make(chan event.Event)

	var wg sync.WaitGroup
	wg.Add(len(subs))
	go func() {
		wg.Wait()
		close(out)
	}()

	for _, sub := range subs {
		sub := sub
		go func() {
			defer wg.Done()

			for msg := range sub.msgs {
				var env envelope
				dec := gob.NewDecoder(bytes.NewReader(msg))
				if err := dec.Decode(&env); err != nil {
					sub.logError(ctx, fmt.Errorf("gob decode envelope: %w", err))
					continue
				}

				data, err := bus.enc.Decode(bytes.NewReader(env.Data), env.Name)
				if err != nil {
					sub.logError(ctx, fmt.Errorf("decode event data: %w [event=%v]", err, env.Name))
					continue
				}

				evt := event.New(
					env.Name,
					data,
					event.ID(env.ID),
					event.Time(env.Time),
					event.Aggregate(
						env.AggregateName,
						env.AggregateID,
						env.AggregateVersion,
					),
				)

				var timeout <-chan time.Time
				var stop func() bool = func() bool { return false }
				if bus.pullTimeout != 0 {
					timer := time.NewTimer(bus.pullTimeout)
					timeout = timer.C
					stop = timer.Stop
				}

				drop := func() {
					stop()
					sub.logError(ctx, fmt.Errorf(
						"event dropped: %w [event=%v, timeout=%v]",
						ErrPullTimeout,
						evt.Name(),
						bus.pullTimeout,
					))
				}

				select {
				case <-timeout:
					drop()
				case out <- evt:
					stop()
				}
			}
		}()
	}

	return out
}

func fanInErrors(subs []*subscription) <-chan error {
	out := make(chan error)

	var wg sync.WaitGroup
	wg.Add(len(subs))
	go func() {
		wg.Wait()
		close(out)
	}()

	for _, sub := range subs {
		errs := sub.errs
		go func() {
			defer wg.Done()
			for err := range errs {
				out <- err
			}
		}()
	}

	return out
}

func (sub *subscription) logError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
	case sub.errs <- err:
	}
}

func discardErrors(errs <-chan error) {
	for range errs {
	}
}

// Uses the event name as the subject but replace "." with "_" because "." is
// not allowed in subjects.
func defaultSubjectFunc(eventName string) string {
	return replaceDots(eventName)
}

// Concatenates the subject and queue name together with an underscore.
// If queue is an empty string, defaultSubjectFunc(subject) is returned.
func defaultDurableNameFunc(subject, queue string) string {
	if queue == "" {
		return subject
	}
	return fmt.Sprintf("%s_%s", subject, queue)
}

func envDurableNameFunc() (func(string, string) string, error) {
	type data struct {
		Subject string
		Queue   string
	}

	nameTpl := os.Getenv("NATS_DURABLE_NAME")
	if nameTpl == "" {
		return nonDurable, nil
	}

	tpl, err := template.New("").Parse(nameTpl)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	return func(subject, queue string) string {
		var buf strings.Builder
		if err := tpl.Execute(&buf, data{Subject: subject, Queue: queue}); err != nil {
			log.Printf("[nats.EventBus] Failed to execute template on `NATS_DURABLE_NAME` environment variable: %v", err)
			log.Printf("[nats.EventBus] Falling back to non-durable subscription.")

			return nonDurable(subject, queue)
		}
		return buf.String()
	}, nil
}

func nonDurable(string, string) string { return "" }

func noQueue(string) (q string) { return }
