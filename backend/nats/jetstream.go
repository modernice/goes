package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
)

// JetStream returns the NATS JetStream Driver:
//
//	bus := NewEventBus(enc, Use(JetStream()))
//
// Consumer Subscriptions
//
// Consumer subscriptions are given the following options by default:
//	- DeliverPolicy: DeliverNew
//	- AckPolicy: AckAll
//
// You can add custom options using the SubOpts option:
//
//	bus := NewEventBus(enc, Use(JetStream()), SubOpts(nats.DeliverAll(), nats.AckNone()))
func JetStream[ID goes.ID]() Driver[ID] {
	return &jetStream[ID]{
		subs: make(map[string]*subscription[ID]),
	}
}

const jetStreamDriverName = "jetstream"

type jetStream[ID goes.ID] struct {
	sync.RWMutex

	ctx  nats.JetStreamContext
	subs map[string]*subscription[ID]
}

func (js *jetStream[ID]) name() string { return jetStreamDriverName }

func (js *jetStream[ID]) init(bus *EventBus[ID]) (err error) {
	js.ctx, err = bus.conn.JetStream()
	if err != nil {
		err = fmt.Errorf("get JetStreamContext: %w", err)
	}
	return
}

func (js *jetStream[ID]) subscribe(ctx context.Context, bus *EventBus[ID], event string) (recipient[ID], error) {
	// If a subscription for that event already exists, return it.
	if sub, ok := js.subscription(event); ok {
		return sub.subscribe(ctx)
	}

	msgs := make(chan []byte)

	subject := bus.subjectFunc(event)
	queue := bus.queueFunc(event)
	durableName := bus.durableFunc(subject, queue)

	stream := streamName(bus, event, subject, queue)

	// Check if the subscription was created by another subscriber in the
	// meantime and return the subscription if it exists.
	js.Lock()
	defer js.Unlock()
	if sub, ok := js.subs[event]; ok {
		return sub.subscribe(ctx)
	}

	if err := js.ensureStream(ctx, stream, subject); err != nil {
		return recipient[ID]{}, fmt.Errorf("ensure stream: %w", err)
	}

	opts := []nats.SubOpt{
		nats.BindStream(stream),
		nats.DeliverNew(),
		nats.AckAll(),
	}
	if durableName != "" {
		opts = append(opts, nats.Durable(durableName))
	}

	opts = append(opts, bus.subOpts...)

	var nsub *nats.Subscription
	handleMsg := func(msg *nats.Msg) { msgs <- msg.Data }

	if queue != "" {
		var err error
		nsub, err = js.ctx.QueueSubscribe(subject, queue, handleMsg, opts...)
		if err != nil {
			return recipient[ID]{}, fmt.Errorf("subscribe with queue group: %w [event=%v, subject=%v, queue=%v, consumer=%v]", err, event, subject, queue, durableName)
		}
	} else {
		var err error
		nsub, err = js.ctx.Subscribe(subject, handleMsg, opts...)
		if err != nil {
			return recipient[ID]{}, fmt.Errorf("subscribe: %w [event=%v, subject=%v, queue=%v, consumer=%v]", err, event, subject, queue, durableName)
		}
	}

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

func (js *jetStream[ID]) ensureStream(ctx context.Context, streamName, subject string) error {
	_, err := js.ctx.StreamInfo(streamName)
	if err == nil {
		// TODO(bounoable): Validate the stream config and return an error if it
		// doesn't match.
		return nil
	}

	if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("get stream info: %w [stream=%v]", err, streamName)
	}

	subjects := []string{subject}

	if _, err := js.ctx.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: subjects,
	}); err != nil {
		return fmt.Errorf("add stream: %w [name=%v, subjects=%v]", err, streamName, subjects)
	}

	return nil
}

func (js *jetStream[ID]) publish(ctx context.Context, bus *EventBus[ID], evt event.Of[any, ID]) error {
	var buf bytes.Buffer
	if err := bus.enc.Encode(&buf, evt.Name(), evt.Data()); err != nil {
		return fmt.Errorf("encode event data: %w [event=%v, type(data)=%T]", err, evt.Name(), evt.Data())
	}

	b := buf.Bytes()

	id, name, v := evt.Aggregate()

	env := envelope[ID]{
		ID:               evt.ID(),
		Name:             evt.Name(),
		Time:             evt.Time(),
		Data:             b,
		AggregateName:    name,
		AggregateID:      id,
		AggregateVersion: v,
	}

	buf = bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(env); err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}

	subject := bus.subjectFunc(env.Name)
	queue := bus.queueFunc(evt.Name())
	streamName := streamName(bus, evt.Name(), subject, queue)

	if err := js.ensureStream(ctx, streamName, subject); err != nil {
		return fmt.Errorf("ensure stream: %w", err)
	}

	var opts []nats.PubOpt
	var zero ID
	if id := evt.ID(); id != zero {
		opts = append(opts, nats.MsgId(id.String()))
	}

	if _, err := js.ctx.Publish(subject, buf.Bytes(), opts...); err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	return nil
}

func (js *jetStream[ID]) subscription(event string) (*subscription[ID], bool) {
	js.RLock()
	defer js.RUnlock()
	sub, ok := js.subs[event]
	return sub, ok
}

// replaces illegal stream name characters with "_".
func streamName[ID goes.ID](bus *EventBus[ID], event, subject, queue string) string {
	// bus.streamNameFunc uses either the user-provided StreamNameFunc option to
	// generate the stream names or falls back to the defaultStreamNameFunc,
	// which just returns the subject as it is.
	name := replacer.Replace(bus.streamNameFunc(subject, queue))

	// If the user provided a StreamNameFunc that returns an empty string, we
	// fall back to the defaultStreamNameFunc and print a warning that the
	// option was overriden.
	if name == "" {
		name = replacer.Replace(defaultStreamNameFunc(subject, queue))

		log.Printf(
			"[goes/backend/nats.jetStream] User-provided StreamNameFunc returned an empty string. "+
				"Using default stream name %q. [event=%v, subject=%v, queue=%v]",
			name, event, subject, queue,
		)
	}

	return name
}
