// Package nats provides an event bus that uses NATS to publish and subscribe to
// events over a network with support for both NATS Core and NATS JetStream.
package nats

import (
	"context"
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
	stop        chan struct{}
}

// EventBusOption is an option for an EventBus.
type EventBusOption func(*EventBus)

// A Driver provides the specific implementation for interacting with either
// NATS Core or NATS JetStream. Use the Core or JetStream functions to create
// a Driver.
type Driver interface {
	name() string
	subscribe(ctx context.Context, bus *EventBus, subject string) (recipient, error)
	publish(ctx context.Context, bus *EventBus, evt event.EventOf[any]) error
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
		enc = event.NewRegistry()
	}

	bus := &EventBus{enc: enc, stop: make(chan struct{})}
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

// Disconnect closes the underlying *nats.Conn. Should ctx be canceled before
// the connection is closed, ctx.Err() is returned.
func (bus *EventBus) Disconnect(ctx context.Context) error {
	if bus.conn == nil {
		return nil
	}

	closed := make(chan struct{})
	bus.conn.SetClosedHandler(func(*nats.Conn) { close(closed) })
	bus.conn.Close()

	select {
	case <-ctx.Done():
		bus.conn = nil
		return ctx.Err()
	case <-closed:
		close(bus.stop)
		bus.conn = nil
		return nil
	}
}

// Publish publishes events.
func (bus *EventBus) Publish(ctx context.Context, events ...event.EventOf[any]) error {
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
func (bus *EventBus) Subscribe(ctx context.Context, names ...string) (<-chan event.EventOf[any], <-chan error, error) {
	if err := bus.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}

	rcpts := make([]recipient, len(names))

	for i, name := range names {
		rcpt, err := bus.driver.subscribe(ctx, bus, name)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", bus.driver.name(), err)
		}
		rcpts[i] = rcpt
	}

	if bus.eatErrors {
		discardErrors(rcpts...)
	}

	return bus.fanInEvents(rcpts), fanInErrors(rcpts), nil
}

func (bus *EventBus) init(opts ...EventBusOption) error {
	var envOpts []EventBusOption
	if env.Bool("NATS_QUEUE_GROUP_BY_EVENT") {
		envOpts = append(envOpts, QueueGroupByEvent())
	}

	if queue := env.String("NATS_QUEUE_GROUP"); queue != "" {
		envOpts = append(envOpts, QueueGroup(queue))
	}

	if service := env.String("NATS_LOAD_BALANCER"); service != "" {
		envOpts = append(envOpts, WithLoadBalancer(service))
	}

	if prefix := strings.TrimSpace(env.String("NATS_SUBJECT_PREFIX")); prefix != "" {
		envOpts = append(envOpts, SubjectPrefix(prefix))
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

	// Note: Only used by JetStream driver.
	if bus.streamNameFunc == nil {
		bus.streamNameFunc = defaultStreamNameFunc
	}

	if bus.durableFunc == nil {
		fn, err := envDurableNameFunc()
		if err != nil {
			return fmt.Errorf("parse durable name from env: %w", err)
		}
		bus.durableFunc = fn
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

func (bus *EventBus) fanInEvents(rcpts []recipient) <-chan event.EventOf[any] {
	out := make(chan event.EventOf[any])

	var wg sync.WaitGroup
	wg.Add(len(rcpts))
	go func() {
		wg.Wait()
		close(out)
	}()

	drop := func(rcpt recipient, evt event.EventOf[any]) {
		rcpt.log(fmt.Errorf(
			"[goes/backend/nats.EventBus] event dropped: %w [event=%v, timeout=%v]",
			ErrPullTimeout,
			evt.Name(),
			bus.pullTimeout,
		))
	}

	for _, rcpt := range rcpts {
		go func(rcpt recipient) {
			defer wg.Done()
			for evt := range rcpt.events {
				var timeout <-chan time.Time
				stop := func() bool { return false }
				if bus.pullTimeout > 0 {
					timer := time.NewTimer(bus.pullTimeout)
					timeout = timer.C
				}

				select {
				case <-rcpt.unsubbed:
					return
				case <-timeout:
					drop(rcpt, evt)
					stop()
				case out <- evt:
					stop()
				}
			}
		}(rcpt)
	}

	return out
}

func fanInErrors(rcpts []recipient) <-chan error {
	out := make(chan error)

	var wg sync.WaitGroup
	wg.Add(len(rcpts))
	go func() {
		wg.Wait()
		close(out)
	}()

	for _, rcpt := range rcpts {
		go func(rcpt recipient) {
			defer wg.Done()
			for err := range rcpt.errs {
				select {
				case <-rcpt.unsubbed:
					return
				case out <- err:
				}
			}
		}(rcpt)
	}

	return out
}

func discardErrors(rcpts ...recipient) {
	for _, rcpt := range rcpts {
		go func(rcpt recipient) {
			for range rcpt.errs {
			}
		}(rcpt)
	}
}

func defaultSubjectFunc(eventName string) string {
	return replaceDots(eventName)
}

// // Concatenates the subject and queue name together with an underscore.
// // If queue is an empty string, defaultSubjectFunc(subject) is returned.
// func defaultDurableNameFunc(subject, queue string) string {
// 	if queue == "" {
// 		return replaceDots(subject)
// 	}
// 	return replaceDots(fmt.Sprintf("%s_%s", subject, queue))
// }

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
			log.Printf("[goes/backend/nats.EventBus] Failed to execute template on `NATS_DURABLE_NAME` environment variable: %v", err)
			log.Printf("[goes/backend/nats.EventBus] Falling back to non-durable subscription.")

			return nonDurable(subject, queue)
		}
		return buf.String()
	}, nil
}

func nonDurable(string, string) string { return "" }

func noQueue(string) (q string) { return }

// Just returns the subject. If the user provides a custom streamNameFunc, the
// queue name is provided, but we don't want to use it here because subjects are
// always just event names and we cannot create multiple JetStream streams with
// the same subjects, so using the queue group here would not really work.
func defaultStreamNameFunc(subject, _ string) string {
	return subject
}
