// Package nats provides an event.Bus implementation using a NATS client for transport.
package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/env"
	"github.com/nats-io/nats.go"
)

// EventBus is the NATS event.Bus implementation.
type EventBus struct {
	enc            event.Encoder
	queueFunc      func(string) string
	subjectFunc    func(string) string
	url            string
	connectOpts    []nats.Option
	receiveTimeout time.Duration

	connMux sync.Mutex
	conn    *nats.Conn
	subs    map[subscriber]struct{}

	errs                 chan error
	initErrors           []error
	errorHandlingStarted chan struct{}
	errSubsMux           sync.RWMutex
	errSubs              []errorSubscriber

	onceConnect    sync.Once
	onceErrors     sync.Once
	onceInitErrors sync.Once
}

// Option is an EventBus option.
type Option func(*EventBus)

type subscriber struct {
	msgs chan *nats.Msg
	sub  *nats.Subscription
	done chan struct{}
}

type errorSubscriber struct {
	ctx  context.Context
	errs chan error
	mux  *sync.Mutex
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

// QueueGroupByFunc returns an Option that sets the NATS queue group for
// subscriptions by calling fn with the name of the subscribed Event. This can
// be used to load-balance Events between subscribers.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/queue
func QueueGroupByFunc(fn func(eventName string) string) Option {
	return func(bus *EventBus) {
		bus.queueFunc = fn
	}
}

// QueueGroupByEvent returns an Option that sets the NATS queue group for
// subscriptions to the name of the handled Event. This can be used to
// load-balance Events between subscribers of the same Event name.
//
// Can also be set with the "NATS_QUEUE_GROUP_BY_EVENT" environment variable.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/queue
func QueueGroupByEvent() Option {
	return QueueGroupByFunc(func(eventName string) string {
		return eventName
	})
}

// SubjectFunc returns an Option that sets the NATS subject for subscriptions
// and outgoing Events by calling fn with the name of the handled Event.
func SubjectFunc(fn func(eventName string) string) Option {
	return func(bus *EventBus) {
		bus.subjectFunc = fn
	}
}

// SubjectPrefix returns an Option that sets the NATS subject for subscriptions
// and outgoing Events by prepending prefix to the name of the handled Event.
//
// Can also be set with the "NATS_SUBJECT_PREFIX" environment variable.
func SubjectPrefix(prefix string) Option {
	return SubjectFunc(func(eventName string) string {
		return prefix + eventName
	})
}

// ConnectWith returns an Option that adds custom nats.Options when connecting
// to NATS. Connection to NATS will be established on the first call to
// bus.Publish or bus.Subscribe.
func ConnectWith(opts ...nats.Option) Option {
	return func(bus *EventBus) {
		bus.connectOpts = append(bus.connectOpts, opts...)
	}
}

// URL returns an Option that sets the connection URL to the NATS server. If no
// URL is specified, the environment variable "NATS_URL" will be used as the
// connection URL.
//
// Can also be set with the "NATS_URL" environment variable.
func URL(url string) Option {
	return func(bus *EventBus) {
		bus.url = url
	}
}

// Connection returns an Option that provides the underlying nats.Conn for the
// EventBus.
func Connection(conn *nats.Conn) Option {
	return func(bus *EventBus) {
		bus.conn = conn
	}
}

// ReceiveTimeout returns an Option that limits the duration the EventBus tries
// to send Events into the channel returned by bus.Subscribe. When d is exceeded
// the Event will be dropped and an error will be sent to channels returned by
// bus.Errors. A duration of 0 means no timeout and is the default.
//
// Can also be set with the "NATS_RECEIVE_TIMEOUT" environment variable in a
// format understood by time.ParseDuration. If the environment value is not
// parseable by time.ParseDuration, no timeout will be used and an error will be
// sent to the first channel(s) returned by bus.Errors.
func ReceiveTimeout(d time.Duration) Option {
	return func(bus *EventBus) {
		bus.receiveTimeout = d
	}
}

// New returns a new EventBus that encodes and decodes event.Data using the
// provided Encoder. New panics if enc is nil.
func New(enc event.Encoder, opts ...Option) *EventBus {
	if enc == nil {
		panic("missing encoder")
	}
	bus := EventBus{
		enc:                  enc,
		subs:                 make(map[subscriber]struct{}),
		errs:                 make(chan error),
		errorHandlingStarted: make(chan struct{}),
	}
	bus.init(opts...)

	return &bus
}

func (bus *EventBus) init(opts ...Option) {
	var envOpts []Option
	if env.Bool("NATS_QUEUE_GROUP_BY_EVENT") {
		envOpts = append(envOpts, QueueGroupByEvent())
	}

	if prefix := strings.TrimSpace(env.String("NATS_SUBJECT_PREFIX")); prefix != "" {
		envOpts = append(envOpts, SubjectPrefix(prefix))
	}

	if env.String("NATS_RECEIVE_TIMEOUT") != "" {
		if d, err := env.Duration("NATS_RECEIVE_TIMEOUT"); err == nil {
			envOpts = append(envOpts, ReceiveTimeout(d))
		} else {
			bus.initErrors = append(bus.initErrors, fmt.Errorf(
				"parse environment variable %q: %w",
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
		bus.subjectFunc = defaultSubject
	}
}

// Publish sends each Event evt in events to subscribers who subscribed to
// Events with a name of evt.Name().
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

	subject := bus.subjectFunc(env.Name)
	if err := bus.conn.Publish(subject, buf.Bytes()); err != nil {
		return fmt.Errorf("nats: %w", err)
	}

	return nil
}

// Subscribe returns a channel of Events. For every published Event evt where
// evt.Name() is one of names, that Event will be received from the returned
// Event channel. When ctx is canceled, events will be closed.
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
		var (
			s    *nats.Subscription
			err  error
			msgs = make(chan *nats.Msg)
		)

		subject := bus.subjectFunc(name)

		if group := bus.queueFunc(name); group != "" {
			s, err = bus.conn.ChanQueueSubscribe(subject, group, msgs)
		} else {
			s, err = bus.conn.ChanSubscribe(subject, msgs)
		}

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
	if subscribeError != nil {
		var cancel context.CancelFunc
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
	// user provided a nats.Conn
	if bus.conn != nil {
		return nil
	}

	connectError := make(chan error)
	go func() {
		var err error
		uri := bus.natsURL()
		if bus.conn, err = nats.Connect(uri, bus.connectOpts...); err != nil {
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

func (bus *EventBus) natsURL() string {
	url := nats.DefaultURL
	if bus.url != "" {
		url = bus.url
	} else if envuri := os.Getenv("NATS_URL"); envuri != "" {
		url = envuri
	}
	return url
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
			defer close(sub.done)
			for msg := range sub.msgs {
				var env envelope
				dec := gob.NewDecoder(bytes.NewReader(msg.Data))
				if err := dec.Decode(&env); err != nil {
					bus.error(fmt.Errorf("gob decode envelope: %w", err))
					continue
				}

				data, err := bus.enc.Decode(env.Name, bytes.NewReader(env.Data))
				if err != nil {
					bus.error(
						fmt.Errorf(`encode %q event data: %w`, env.Name, err),
					)
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

				if bus.receiveTimeout == 0 {
					events <- evt
					continue
				}

				timer := time.NewTimer(bus.receiveTimeout)
				select {
				case <-timer.C:
					bus.error(fmt.Errorf(
						"dropping %q event because it wasn't received after %s",
						env.Name,
						bus.receiveTimeout,
					))
				case events <- evt:
					timer.Stop()
				}
			}
		}(sub)
	}
	return events
}

// handleUnsubscribe unsubscribes subs when ctx is canceled
func (bus *EventBus) handleUnsubscribe(ctx context.Context, subs ...subscriber) {
	<-ctx.Done()
	for _, sub := range subs {
		if err := sub.sub.Unsubscribe(); err != nil {
			bus.error(fmt.Errorf(
				`unsubscribe from subject "%s": %w`,
				sub.sub.Subject,
				err,
			))
		}
		close(sub.msgs)
	}
}

// Errors returns an error channel that receives future asynchronous errors from
// the EventBus. When ctx is canceled, the error channel wil be closed.
func (bus *EventBus) Errors(ctx context.Context) <-chan error {
	errs := make(chan error)

	select {
	case <-ctx.Done():
		close(errs)
		return errs
	default:
	}

	// start sending errors to subscribers on first subscription
	bus.onceErrors.Do(bus.goHandleErrors)

	sub := errorSubscriber{
		ctx:  ctx,
		errs: errs,
		mux:  &sync.Mutex{},
	}

	bus.errSubsMux.Lock()
	bus.errSubs = append(bus.errSubs, sub)
	bus.errSubsMux.Unlock()

	go bus.handleErrorUnsubscribe(sub)

	// publish errors that happened in bus.init
	bus.onceInitErrors.Do(bus.publishInitErrors)

	return errs
}

func (bus *EventBus) goHandleErrors() {
	go bus.handleErrors()
}

func (bus *EventBus) handleErrors() {
	close(bus.errorHandlingStarted)
	for err := range bus.errs {
		bus.errSubsMux.RLock()
		for _, sub := range bus.errSubs {
			go func(sub errorSubscriber, err error) {
				sub.mux.Lock()
				defer sub.mux.Unlock()

				select {
				case <-sub.ctx.Done():
				case sub.errs <- err:
				}
			}(sub, err)
		}
		bus.errSubsMux.RUnlock()
	}
}

func (bus *EventBus) handleErrorUnsubscribe(sub errorSubscriber) {
	// close the subscription's error channel when done
	defer sub.close()

	// wait until sub.ctx is canceled
	<-sub.ctx.Done()

	// remove sub from subscribers
	bus.errSubsMux.Lock()
	defer bus.errSubsMux.Unlock()

	for i, errSub := range bus.errSubs {
		if sub == errSub {
			bus.errSubs = append(bus.errSubs[:i], bus.errSubs[i+1:]...)
		}
	}
}

func (bus *EventBus) publishInitErrors() {
	go func() {
		for _, err := range bus.initErrors {
			bus.error(fmt.Errorf("init: %w", err))
		}
	}()
}

func (bus *EventBus) error(err error) {
	select {
	case <-bus.errorHandlingStarted:
		bus.errs <- err
	default:
	}
}

func (sub errorSubscriber) close() {
	sub.mux.Lock()
	close(sub.errs)
	sub.mux.Unlock()
}

// noQueue is a no-op that always returns an empty string. It's used as the
// default queue group function and prevents queue groups from being used
func noQueue(string) (q string) {
	return
}

func defaultSubject(eventName string) string {
	return eventName
}
