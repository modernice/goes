package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
)

const jetStreamDriverName = "jetstream"

var (
	// DefaultStream is the default JetStream stream name to use/create if no
	// explicit name is provided using the StreamName() option.
	DefaultStream = "goes"
)

var (
	// ErrStreamExists is returned when the JetStream driver tries to create a
	// stream that already exists with a different configuration.
	ErrStreamExists = errors.New("stream already exists with a different configuration")

	// ErrConsumerExists is returned when the JetStream driver tries to create a
	// consumer that already exists with a different configuration.
	ErrConsumerExists = errors.New("consumer already exists with a different configuration")
)

// JetStreamOption is an option for the JetStream driver.
type JetStreamOption func(*jetStream)

// StreamName returns a JetStreamOption that specifies the stream name that is
// created by the JetStream driver. The default stream name is "goes".
func StreamName(stream string) JetStreamOption {
	return func(js *jetStream) {
		js.stream = stream
	}
}

// DurableFunc returns an option that makes JetStream consumers / subscriptions
// durable. When creating a consumer, the provided function is called with the
// event name and queue group (see QueueGroup and LoadBalancer options) to
// generate the durable name. If the event is the wildcard "*", it is passed as
// "$all". Similarly, if the queue group is an empty string, it is passed as
// "$noqueue". Any ".", "*", or ">" characters in the returned durable name will
// be replaced by "_".
//
// The JetStream driver creates one consumer / subscription per event.
//
// Read more about durable subscriptions:
// https://docs.nats.io/nats-concepts/jetstream/consumers#durable-name
func DurableFunc(fn func(event, queue string) string) JetStreamOption {
	return func(js *jetStream) {
		js.durableFunc = func(event, queue string) string {
			return replacer.Replace(fn(event, queue))
		}
	}
}

// Durable returns an option that makes JetStream consumers / subscriptions
// durable (see DurableFunc). The durable name is formatted like this:
//	fmt.Sprintf("%s:%s:%s", prefix, queue, event)
func Durable(prefix string) JetStreamOption {
	return DurableFunc(func(event, queue string) string {
		return fmt.Sprintf("%s:%s:%s", prefix, queue, event)
	})
}

// SubOpts returns an option that adds custom nats.SubOpts when creating
// a JetStream subscription.
func SubOpts(opts ...nats.SubOpt) JetStreamOption {
	return func(js *jetStream) {
		js.subOpts = append(js.subOpts, opts...)
	}
}

// JetStream returns the NATS JetStream Driver:
//
//	bus := NewEventBus(enc, Use(JetStream()))
func JetStream(opts ...JetStreamOption) Driver {
	js := &jetStream{
		subs: make(map[string]*subscription),
	}
	for _, opt := range opts {
		opt(js)
	}

	if js.stream == "" {
		js.stream = DefaultStream
	}

	if js.durableFunc == nil {
		js.durableFunc = nonDurable
	}

	return js
}

type jetStream struct {
	sync.RWMutex

	stream      string
	subOpts     []nats.SubOpt
	durableFunc func(subject string, queue string) string

	ctx  nats.JetStreamContext
	subs map[string]*subscription
}

func (js *jetStream) name() string { return jetStreamDriverName }

func (js *jetStream) init(bus *EventBus) (err error) {
	js.ctx, err = bus.conn.JetStream()
	if err != nil {
		err = fmt.Errorf("get JetStreamContext: %w", err)
	}
	return
}

func (js *jetStream) subscribe(ctx context.Context, bus *EventBus, event string) (recipient, error) {
	// If a subscription already exists for the event, return it.
	if sub, ok := js.subscription(event); ok {
		return sub.subscribe(ctx)
	}

	msgs := make(chan []byte)

	// Check if the subscription was created by another subscriber in the
	// meantime and return the subscription if it exists.
	js.Lock()
	defer js.Unlock()
	if sub, ok := js.subs[event]; ok {
		return sub.subscribe(ctx)
	}

	if err := js.ensureStream(ctx); err != nil {
		return recipient{}, fmt.Errorf("ensure stream: %w", err)
	}

	queue := bus.queueFunc(normalizeEvent(event))
	durableName := js.durableFunc(normalizeEvent(event), normalizeQueue(queue))

	userProvidedSubject := bus.subjectFunc(event)
	subject := subscribeSubject(userProvidedSubject, event)

	// By default, we let JetStream create an ephemeral consumer. If the user
	// provides a durable name or queue group, we create a durable consumer.
	var consumerName string
	if durableName != "" || queue != "" {
		consumerName = jsConsumerName(durableName, queue, event)
		if err := js.ensureConsumer(ctx, bus, consumerName, event, subject, queue); err != nil {
			return recipient{}, fmt.Errorf("ensure consumer: %w", err)
		}
	}

	nsub, err := js.natsSubscribe(
		ctx,
		msgs,
		event,
		subject,
		queue,
		consumerName,
		js.makeSubOpts(consumerName)...,
	)
	if err != nil {
		return recipient{}, err
	}

	return js.addRecipient(ctx, bus, event, nsub, msgs)
}

func (js *jetStream) natsSubscribe(
	ctx context.Context,
	msgs chan<- []byte,
	event,
	subject,
	queue,
	consumerName string,
	opts ...nats.SubOpt,
) (*nats.Subscription, error) {
	handleMsg := func(msg *nats.Msg) {
		select {
		case <-ctx.Done():
		case msgs <- msg.Data:
		}
	}

	var nsub *nats.Subscription

	var err error
	if queue != "" {
		if nsub, err = js.ctx.QueueSubscribe(subject, queue, handleMsg, opts...); err != nil {
			err = fmt.Errorf(
				"subscribe: %w [event=%v, subject=%v, queue=%v, consumer=%v, mode=push]",
				err, event, subject, queue, consumerName,
			)
		}
	} else {
		if nsub, err = js.ctx.Subscribe(subject, handleMsg, opts...); err != nil {
			err = fmt.Errorf(
				"subscribe: %w [event=%v, subject=%v, queue=%v, consumer=%v, mode=push]",
				err, event, subject, queue, consumerName,
			)
		}
	}

	if err != nil {
		return nsub, err
	}

	if err := nsub.SetPendingLimits(-1, -1); err != nil {
		return nsub, fmt.Errorf("SetPendingLimits(-1, -1) on nats subscription: %w", err)
	}

	return nsub, nil
}

func (js *jetStream) addRecipient(ctx context.Context, bus *EventBus, event string, nsub *nats.Subscription, msgs chan []byte) (recipient, error) {
	sub := newSubscription(event, bus, nsub, msgs)
	js.subs[event] = sub

	rcpt, err := sub.subscribe(ctx)
	if err != nil {
		return rcpt, err
	}

	go func() {
		<-sub.stop
		js.Lock()
		defer js.Unlock()
		if jssub, ok := js.subs[event]; ok && jssub == sub {
			delete(js.subs, event)
		}
	}()

	return rcpt, nil
}

func (js *jetStream) publish(ctx context.Context, bus *EventBus, evt event.Event) error {
	b, err := bus.enc.Marshal(evt.Data())
	if err != nil {
		return fmt.Errorf("encode event data: %w [event=%v, type(data)=%T]", err, evt.Name(), evt.Data())
	}

	id, name, v := evt.Aggregate()

	env := envelope{
		ID:               evt.ID(),
		Name:             evt.Name(),
		Time:             evt.Time(),
		Data:             b,
		AggregateName:    name,
		AggregateID:      id,
		AggregateVersion: v,
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(env); err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}

	subject := bus.subjectFunc(env.Name)

	var opts []nats.PubOpt
	if id := evt.ID(); id != uuid.Nil {
		opts = append(opts, nats.MsgId(id.String()))
	}

	if _, err := js.ctx.Publish(subject, buf.Bytes(), opts...); err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	return nil
}

func (js *jetStream) ensureStream(ctx context.Context) error {
	info, err := js.ctx.StreamInfo(js.stream)
	if err == nil {
		if info.Config.Name != js.stream {
			return fmt.Errorf("%w: stream name mismatch: %q != %q", ErrStreamExists, info.Config.Name, js.stream)
		}

		if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != "*" {
			return fmt.Errorf("%w: subjects mismatch: %v != %v", ErrStreamExists, info.Config.Subjects, []string{"*"})
		}

		return nil
	}

	if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("get stream info: %w [stream=%v]", err, js.stream)
	}

	subjects := []string{"*"}

	if _, err := js.ctx.AddStream(&nats.StreamConfig{
		Name:     js.stream,
		Subjects: subjects,
	}); err != nil {
		return fmt.Errorf("add stream: %w [name=%v, subjects=%v]", err, js.stream, subjects)
	}

	return nil
}

func (js *jetStream) ensureConsumer(ctx context.Context, bus *EventBus, name, eventName, subject, queue string) error {
	if info, err := js.ctx.ConsumerInfo(js.stream, name); err == nil {
		if info.Stream != js.stream {
			return fmt.Errorf("%w: stream name mismatch: %q != %q", ErrConsumerExists, info.Stream, js.stream)
		}

		if info.Config.FilterSubject != subject {
			return fmt.Errorf("%w: subject mismatch: %q != %q", ErrConsumerExists, info.Config.FilterSubject, subject)
		}

		return nil
	}

	deliverSubject := jsDeliverSubject(bus, eventName, name)

	cfg := nats.ConsumerConfig{
		Durable:        name,
		DeliverSubject: deliverSubject,
		DeliverPolicy:  nats.DeliverAllPolicy,
		DeliverGroup:   queue,
		AckPolicy:      nats.AckAllPolicy,
		FilterSubject:  subject,
	}

	if _, err := js.ctx.AddConsumer(js.stream, &cfg); err != nil {
		return err
	}

	return nil
}

func (js *jetStream) subscription(event string) (*subscription, bool) {
	js.RLock()
	defer js.RUnlock()
	sub, ok := js.subs[event]
	return sub, ok
}

func (js *jetStream) makeSubOpts(consumerName string) []nats.SubOpt {
	// Bind to an existing consumer.
	if consumerName != "" {
		return append([]nats.SubOpt{nats.Bind(js.stream, consumerName)}, js.subOpts...)
	}

	// Let NATS create an ephemeral consumer.
	return append([]nats.SubOpt{
		nats.BindStream(js.stream),
		nats.DeliverNew(),
		nats.AckAll(),
	}, js.subOpts...)
}

func jsConsumerName(durable, queue, eventName string) string {
	if durable != "" {
		return durable
	}
	eventName = normalizeEvent(eventName)
	if queue == "" {
		return replacer.Replace(eventName)
	}
	return replacer.Replace(fmt.Sprintf("%s_%s", queue, eventName))
}

func jsDeliverSubject(bus *EventBus, event, consumer string) string {
	event = normalizeEvent(event)
	subject := bus.subjectFunc(event)
	return fmt.Sprintf("%s.%s.deliver", consumer, subject)
}

func normalizeEvent(event string) string {
	if event == "*" {
		return "$all"
	}
	return event
}

func normalizeQueue(queue string) string {
	if queue == "" {
		return "$noqueue"
	}
	return queue
}

func nonDurable(string, string) string { return "" }
