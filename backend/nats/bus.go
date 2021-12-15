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
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/env"
	"github.com/nats-io/nats.go"
)

var (
	// ErrReceiveTimeout is returned when an Event is not received from a
	// subscriber Event channel after the configured ReceiveTimeout.
	ErrReceiveTimeout = errors.New("receive timed out")
)

type EventBus struct {
	enc event.Encoder

	eatErrors      bool
	url            string
	receiveTimeout time.Duration

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

type Driver interface {
	name() string
	subscribe(ctx context.Context, bus *EventBus, subject string) (*subscription, error)
	publish(ctx context.Context, bus *EventBus, evt event.Event) error
}

type EventBusOption func(*EventBus)

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

func EatErrors() EventBusOption {
	return func(bus *EventBus) {
		bus.eatErrors = true
	}
}

// ReceiveTimeout returns an Option that limits the duration the EventBus tries
// to send Events into the channel returned by bus.Subscribe. When d is exceeded
// the Event will be dropped. The default is a duration of 0 and means no timeout.
//
// Can also be set with the "NATS_RECEIVE_TIMEOUT" environment variable in a
// format understood by time.ParseDuration. If the environment value is not
// parseable by time.ParseDuration, no timeout will be used.
func ReceiveTimeout(d time.Duration) EventBusOption {
	return func(bus *EventBus) {
		bus.receiveTimeout = d
	}
}

func NewEventBus(enc event.Encoder, opts ...EventBusOption) *EventBus {
	if enc == nil {
		enc = event.NewRegistry()
	}

	bus := &EventBus{enc: enc}
	for _, opt := range opts {
		opt(bus)
	}
	bus.init()

	return bus
}

func (bus *EventBus) Publish(ctx context.Context, events ...event.Event) error {
	if err := bus.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	for _, evt := range events {
		if err := bus.driver.publish(ctx, bus, evt); err != nil {
			return fmt.Errorf("publish event: %w [event=%v]", err, evt.Name())
		}
	}

	return nil
}

func (bus *EventBus) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, <-chan error, error) {
	if err := bus.connectOnce(ctx); err != nil {
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

	if env.String("NATS_RECEIVE_TIMEOUT") != "" {
		if d, err := env.Duration("NATS_RECEIVE_TIMEOUT"); err == nil {
			envOpts = append(envOpts, ReceiveTimeout(d))
		} else {
			panic(fmt.Errorf(
				"init: parse environment variable %q: %w",
				"NATS_RECEIVE_TIMEOUT",
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

func (bus *EventBus) connectOnce(ctx context.Context) error {
	var err error
	bus.onceConnect.Do(func() {
		if err = bus.connect(ctx); err != nil {
			return
		}

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

				data, err := bus.enc.Decode(env.Name, bytes.NewReader(env.Data))
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
				if bus.receiveTimeout != 0 {
					timer := time.NewTimer(bus.receiveTimeout)
					timeout = timer.C
					stop = timer.Stop
				}

				drop := func() {
					stop()
					sub.logError(ctx, fmt.Errorf("%w: dropping event (timeout) [event=%v, timeout=%v]", ErrReceiveTimeout, evt.Name(), bus.receiveTimeout))
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
	return strings.ReplaceAll(eventName, ".", "_")
}

// Concatenates the subject and queue name together with an underscore.
// If queue is an empty string, defaultSubjectFunc(subject) is returned.
func defaultDurableNameFunc(subject, queue string) string {
	if queue == "" {
		return defaultSubjectFunc(subject)
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
