// Package natsbus provides an event.Bus implementation with support for both
// NATS Core and NATS Streaming as the backend.
package natsbus

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/env"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
)

var (
	// ErrReceiveTimeout is returned when an Event is not received from a
	// subscriber Event channel after the configured ReceiveTimeout.
	ErrReceiveTimeout = errors.New("receive timed out")
)

// Bus is the NATS event.Bus implementation.
type Bus struct {
	driver Driver
	enc    event.Encoder

	eatErrors bool

	queueFunc      func(string) string
	subjectFunc    func(string) string
	durableFunc    func(string, string) string
	url            string
	connectOpts    []nats.Option
	receiveTimeout time.Duration

	connMux sync.Mutex
	conn    connection
	subs    map[subscriber]struct{}

	onceConnect sync.Once
}

// Option is a Bus option.
type Option func(*Bus)

// A Driver connects to a NATS cluster. Available Drivers:
//	- Core() returns the NATS Core Driver (default)
//	- Streaming() returns the NATS Streaming Driver
type Driver interface {
	connect(string) (connection, error)
}

type connection interface {
	get() interface{}
	subscribe(subject string) (subscriber, error)
	queueSubscribe(subject, queue string) (subscriber, error)
	publish(string, []byte) error
}

type core struct{ opts []nats.Option }

type streaming struct {
	clusterID   string
	clientID    string
	opts        []stan.Option
	durableFunc func(string, string) string
}

type natsConn struct{ conn *nats.Conn }

type stanConn struct {
	conn        stan.Conn
	durableFunc func(string, string) string
}

type subscriber struct {
	subject      string
	queue        string
	natsSub      *nats.Subscription
	stanSub      stan.Subscription
	msgs         chan []byte
	events       chan<- event.Event
	errs         chan<- error
	unsubscribed chan struct{}
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

// Use returns an Option that specifies which Driver to use to communicate with
// NATS. Defaults to Core().
func Use(d Driver) Option {
	if d == nil {
		panic("nil Driver")
	}
	return func(bus *Bus) {
		bus.driver = d
	}
}

// QueueGroupByFunc returns an Option that sets the NATS queue group for
// subscriptions by calling fn with the name of the subscribed Event. This can
// be used to load-balance Events between subscribers.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/queue
func QueueGroupByFunc(fn func(eventName string) string) Option {
	return func(bus *Bus) {
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
	return func(bus *Bus) {
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

// DurableFunc returns an Option that sets fn as the function to build the
// DurableName for the NATS Streaming subscriptions. When fn return an empty
// string, the subscription will not be made durable.
//
// DurableFunc has no effect when using the NATS Core Driver because NATS Core
// doesn't support durable subscriptions.
//
// Can also be set with the "NATS_DURABLE_NAME" environment variable:
//	`NATS_DURABLE_NAME={{ .Subject }}_{{ .Queue }}`
//
// Read more about durable subscriptions:
// https://docs.nats.io/developing-with-nats-streaming/durables
func DurableFunc(fn func(subject, queueGroup string) string) Option {
	return func(bus *Bus) {
		bus.durableFunc = fn
	}
}

// Durable returns an Option that makes the NATS subscriptions durable.
//
// If the queue group is not empty, the durable name is built by concatenating
// the subject and queue group with an underscore:
//	fmt.Sprintf("%s_%s", subject, queueGroup)
//
// If the queue group is an empty string, the durable name is set to the
// subject.
//
// Can also be set with the "NATS_DURABLE_NAME" environment variable:
//	`NATS_DURABLE_NAME={{ .Subject }}_{{ .Queue }}`
//
// Use DurableFunc instead to control how the durable name is built.
func Durable() Option {
	return DurableFunc(defaultDurableName)
}

// URL returns an Option that sets the connection URL to the NATS server. If no
// URL is specified, the environment variable "NATS_URL" will be used as the
// connection URL.
//
// Can also be set with the "NATS_URL" environment variable.
func URL(url string) Option {
	return func(bus *Bus) {
		bus.url = url
	}
}

// Conn returns an Option that provides the underlying *nats.Conn for the
// EventBus. When the Conn Option is used, the Use Option has no effect.
func Conn(conn *nats.Conn) Option {
	return func(bus *Bus) {
		bus.conn = &natsConn{conn}
	}
}

// StreamingConn returns an Option that provides the underlying stan.Conn for the
// EventBus. When the StreamingConn Option is used, the Use Option has no effect.
func StreamingConn(conn stan.Conn) Option {
	return func(bus *Bus) {
		bus.conn = &stanConn{
			conn: conn,
		}
	}
}

// ReceiveTimeout returns an Option that limits the duration the EventBus tries
// to send Events into the channel returned by bus.Subscribe. When d is exceeded
// the Event will be dropped and an error will be sent to channels returned by
// bus.Errors. The default is a duration of 0 and means no timeout.
//
// Can also be set with the "NATS_RECEIVE_TIMEOUT" environment variable in a
// format understood by time.ParseDuration. If the environment value is not
// parseable by time.ParseDuration, no timeout will be used and an error will be
// sent to the first channel(s) returned by bus.Errors.
func ReceiveTimeout(d time.Duration) Option {
	return func(bus *Bus) {
		bus.receiveTimeout = d
	}
}

// EatErrors returns an Option that makes the Bus start a goroutine to range
// over and discard any errors from the returned error channel, so that they
// don't have to be received manually if there's no interest in handling those
// errors.
func EatErrors() Option {
	return func(bus *Bus) {
		bus.eatErrors = true
	}
}

// Core returns the NATS Core Driver (at-most-once delivery).
func Core(opts ...nats.Option) Driver {
	return &core{opts}
}

// Streaming returns the NATS Streaming Driver (at-least-once delivery).
func Streaming(clusterID, clientID string, opts ...stan.Option) Driver {
	return &streaming{
		clusterID: clusterID,
		clientID:  clientID,
		opts:      opts,
	}
}

// New returns a new EventBus that encodes and decodes event.Data using the
// provided Encoder.
//
// New panics if enc is nil or initialization fails because of a malformed
// environment variable.
func New(enc event.Encoder, opts ...Option) *Bus {
	if enc == nil {
		panic("nil Encoder")
	}
	bus := Bus{
		enc:  enc,
		subs: make(map[subscriber]struct{}),
	}
	if bus.driver == nil {
		bus.driver = Core(bus.connectOpts...)
	}
	bus.init(opts...)

	return &bus
}

func (bus *Bus) init(opts ...Option) {
	var envOpts []Option
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
		bus.subjectFunc = defaultSubject
	}

	if d, ok := bus.driver.(*streaming); ok && d != nil {
		d.durableFunc = bus.durableFunc
	}

	if c, ok := bus.conn.(*stanConn); ok && c != nil {
		c.durableFunc = bus.durableFunc
	}
}

// Publish implements event.Bus.
func (bus *Bus) Publish(ctx context.Context, events ...event.Event) error {
	if err := bus.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	for _, evt := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := bus.publish(ctx, evt); err != nil {
			return fmt.Errorf(`publish "%s" event: %w`, evt.Name(), err)
		}
	}

	return nil
}

func (bus *Bus) publish(ctx context.Context, evt event.Event) error {
	var buf bytes.Buffer
	if err := bus.enc.Encode(&buf, evt.Name(), evt.Data()); err != nil {
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
	if err := bus.conn.publish(subject, buf.Bytes()); err != nil {
		return fmt.Errorf("nats: %w", err)
	}

	return nil
}

// Subscribe implements event.Bus.
//
// Callers must ensure to range over the error channel if the EatErrors Option
// is not used; otherwise the subscription will block forever and no further
// Events will be received when the first async error happens.
func (bus *Bus) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, <-chan error, error) {
	if err := bus.connectOnce(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	subs := make([]subscriber, 0, len(names))
	out, errs := make(chan event.Event), make(chan error)

	var subscribeError error
	for _, name := range names {
		subject := bus.subjectFunc(name)

		var (
			sub subscriber
			err error
		)

		if group := bus.queueFunc(name); group != "" {
			sub, err = bus.conn.queueSubscribe(subject, group)
		} else {
			sub, err = bus.conn.subscribe(subject)
		}

		if err != nil {
			subscribeError = err
			break
		}

		sub.events = out
		sub.errs = errs

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

	bus.fanIn(out, errs, subs...)

	if bus.eatErrors {
		go drainErrors(errs)
	}

	return out, errs, nil
}

func (bus *Bus) connectOnce(ctx context.Context) error {
	var err error
	bus.onceConnect.Do(func() { err = bus.connect(ctx) })
	return err
}

func (bus *Bus) connect(ctx context.Context) error {
	// user provided a nats.Conn
	if bus.conn != nil {
		return nil
	}

	connectError := make(chan error)
	go func() {
		var err error
		uri := bus.natsURL()

		if bus.conn, err = bus.driver.connect(uri); err != nil {
			connectError <- err
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

func (bus *Bus) natsURL() string {
	url := nats.DefaultURL
	if bus.url != "" {
		url = bus.url
	} else if envuri := os.Getenv("NATS_URL"); envuri != "" {
		url = envuri
	}
	return url
}

func (bus *Bus) fanIn(events chan<- event.Event, errs chan<- error, subs ...subscriber) {
	var wg sync.WaitGroup
	wg.Add(len(subs))

	go func() {
		wg.Wait()
		close(events)
		close(errs)
	}()

	for _, sub := range subs {
		go bus.workSubscriber(sub, &wg)
	}
}

func (bus *Bus) workSubscriber(sub subscriber, wg *sync.WaitGroup) {
	defer wg.Done()
	for msg := range sub.msgs {
		var env envelope
		dec := gob.NewDecoder(bytes.NewReader(msg))
		if err := dec.Decode(&env); err != nil {
			sub.errs <- fmt.Errorf("decode envelope: %w", err)
			continue
		}

		data, err := bus.enc.Decode(env.Name, bytes.NewReader(env.Data))
		if err != nil {
			sub.errs <- fmt.Errorf("encode %q event data: %w", env.Name, err)
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
			sub.events <- evt
			continue
		}

		timer := time.NewTimer(bus.receiveTimeout)
		select {
		case <-timer.C:
			sub.errs <- fmt.Errorf("dropping %q event because: %w", env.Name, ErrReceiveTimeout)
		case sub.events <- evt:
			timer.Stop()
		}
	}
}

// handleUnsubscribe unsubscribes subs when ctx is canceled
func (bus *Bus) handleUnsubscribe(ctx context.Context, subs ...subscriber) {
	<-ctx.Done()
	for _, sub := range subs {
		if err := sub.unsubscribe(); err != nil {
			go func() {
				sub.errs <- fmt.Errorf(
					`unsubscribe from subject "%s": %w`,
					sub.subject,
					err,
				)
			}()
		}
	}
}

func (sub subscriber) unsubscribe() error {
	defer close(sub.unsubscribed)
	if sub.natsSub != nil {
		return sub.natsSub.Unsubscribe()
	}
	return sub.stanSub.Unsubscribe()
}

func (d *core) connect(url string) (connection, error) {
	conn, err := nats.Connect(url, d.opts...)
	if err != nil {
		return nil, fmt.Errorf("nats: %w", err)
	}
	return &natsConn{conn}, nil
}

func (d *streaming) connect(url string) (connection, error) {
	opts := append([]stan.Option{stan.NatsURL(url)}, d.opts...)
	conn, err := stan.Connect(d.clusterID, d.clientID, opts...)
	if err != nil {
		return nil, fmt.Errorf("stan: %w", err)
	}
	return &stanConn{
		conn:        conn,
		durableFunc: d.durableFunc,
	}, nil
}

func (c *natsConn) get() interface{} {
	return c.conn
}

func (c *natsConn) subscribe(subject string) (subscriber, error) {
	msgs := make(chan []byte)
	nmsgs := make(chan *nats.Msg)
	sub, err := c.conn.ChanSubscribe(subject, nmsgs)
	if err != nil {
		return subscriber{}, fmt.Errorf("nats: %w", err)
	}
	go func() {
		defer close(msgs)
		for msg := range nmsgs {
			msgs <- msg.Data
		}
	}()
	s := newSubscriber(subject, "", msgs)
	s.natsSub = sub
	go func() {
		<-s.unsubscribed
		close(nmsgs)
	}()
	return s, nil
}

func newSubscriber(subject, queue string, msgs chan []byte) subscriber {
	return subscriber{
		subject:      subject,
		queue:        queue,
		msgs:         msgs,
		unsubscribed: make(chan struct{}),
	}
}

func (c *natsConn) queueSubscribe(subject, queue string) (subscriber, error) {
	msgs := make(chan []byte)
	nmsgs := make(chan *nats.Msg)
	sub, err := c.conn.ChanQueueSubscribe(subject, queue, nmsgs)
	if err != nil {
		return subscriber{}, fmt.Errorf("nats: %w", err)
	}
	go func() {
		defer close(msgs)
		for msg := range nmsgs {
			msgs <- msg.Data
		}
	}()
	s := newSubscriber(subject, queue, msgs)
	s.natsSub = sub
	go func() {
		<-s.unsubscribed
		close(nmsgs)
	}()
	return s, nil
}

func (c *natsConn) publish(subject string, data []byte) error {
	return c.conn.Publish(subject, data)
}

func (c *stanConn) get() interface{} {
	return c.conn
}

func (c *stanConn) subscribe(subject string) (subscriber, error) {
	msgs := make(chan []byte)
	sub, err := c.conn.Subscribe(
		subject,
		func(msg *stan.Msg) { msgs <- msg.Data },
		stan.DurableName(c.durableFunc(subject, "")),
	)
	if err != nil {
		return subscriber{}, fmt.Errorf("stan: %w", err)
	}
	s := newSubscriber(subject, "", msgs)
	s.stanSub = sub
	go func() {
		<-s.unsubscribed
		close(msgs)
	}()
	return s, nil
}

func (c *stanConn) queueSubscribe(subject, queue string) (subscriber, error) {
	msgs := make(chan []byte)
	sub, err := c.conn.QueueSubscribe(
		subject, queue, func(msg *stan.Msg) { msgs <- msg.Data },
		stan.DurableName(c.durableFunc(subject, queue)),
	)
	if err != nil {
		return subscriber{}, fmt.Errorf("stan: %w", err)
	}
	s := newSubscriber(subject, queue, msgs)
	s.stanSub = sub
	go func() {
		<-s.unsubscribed
		close(msgs)
	}()
	return s, nil
}

func (c *stanConn) publish(subject string, data []byte) error {
	return c.conn.Publish(subject, data)
}

// noQueue is a no-op that always returns an empty string. It's used as the
// default queue group function and prevents queue groups from being used
func noQueue(string) (q string) {
	return
}

func defaultSubject(eventName string) string {
	return eventName
}

func defaultDurableName(subject, queue string) string {
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
	tpl, err := template.New("durableName").Parse(nameTpl)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	return func(subject, queue string) string {
		var buf strings.Builder
		if err := tpl.Execute(&buf, data{Subject: subject, Queue: queue}); err != nil {
			return nonDurable(subject, queue)
		}
		return buf.String()
	}, nil
}

func nonDurable(_, _ string) string {
	return ""
}

func drainErrors(errs <-chan error) {
	var n int
	for range errs {
		n++
	}
}