package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"strings"

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
func JetStream() Driver {
	return &jetStream{}
}

const jetStreamDriverName = "jetstream"

type jetStream struct{ ctx nats.JetStreamContext }

func (jetstream *jetStream) name() string { return jetStreamDriverName }

func (jetstream *jetStream) init(bus *EventBus) (err error) {
	jetstream.ctx, err = bus.conn.JetStream()
	if err != nil {
		err = fmt.Errorf("get JetStreamContext: %w", err)
	}
	return
}

func (jetstream *jetStream) subscribe(ctx context.Context, bus *EventBus, event string) (*subscription, error) {
	msgs := make(chan []byte)
	errs := make(chan error)

	subject := bus.subjectFunc(event)
	queue := bus.queueFunc(event)
	durableName := bus.durableFunc(subject, queue)

	// bus.streamNameFunc uses either the user-provided StreamNameFunc option to
	// generate the stream names or falls back to the defaultStreamNameFunc,
	// which just returns the subject as it is.
	streamName := strings.TrimSpace(bus.streamNameFunc(subject, queue))

	// Now, if the user provided a StreamNameFunc that returns an empty string,
	// we also fall back to the defaultStreamNameFunc and print a warning that
	// the option is overriden.
	if streamName == "" {
		streamName = defaultStreamNameFunc(subject, queue)

		log.Printf(
			"[goes/backend/nats.jetStream] User-provided StreamNameFunc returned an empty string. "+
				"Using default stream name %q. [event=%v, subject=%v, queue=%v]",
			streamName, event, subject, queue,
		)
	}

	// Create the JetStream stream (if it does not exist yet).
	if err := jetstream.ensureStream(ctx, streamName, subject); err != nil {
		return nil, fmt.Errorf("ensure stream: %w", err)
	}

	var sub *nats.Subscription

	handleMsg := func(msg *nats.Msg) { msgs <- msg.Data }

	opts := append([]nats.SubOpt{
		nats.BindStream(streamName),
		nats.Durable(durableName),
		nats.DeliverNew(),
		nats.AckAll(),
	}, bus.subOpts...)

	if queue != "" {
		var err error
		sub, err = jetstream.ctx.QueueSubscribe(subject, queue, handleMsg, opts...)
		if err != nil {
			return nil, fmt.Errorf("subscribe with queue group: %w [subject=%v, queue=%v]", err, subject, queue)
		}
	} else {
		var err error
		sub, err = jetstream.ctx.Subscribe(subject, handleMsg, opts...)
		if err != nil {
			return nil, fmt.Errorf("subscribe: %w [subject=%v]", err, subject)
		}
	}

	unsubbed := make(chan struct{})
	go func() {
		defer close(unsubbed)
		defer close(msgs)
		defer close(errs)
		<-ctx.Done()
		if err := sub.Unsubscribe(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
			log.Printf("[goes/backend/nats.EventBus] unsubscribe: %v [subject=%v, queue=%v]", err, sub.Subject, sub.Queue)
		}
	}()

	return &subscription{
		sub:       sub,
		msgs:      msgs,
		errs:      errs,
		unsubbbed: unsubbed,
	}, nil
}

func (jetstream *jetStream) ensureStream(ctx context.Context, streamName, subject string) error {
	_, err := jetstream.ctx.StreamInfo(streamName)
	if err == nil {
		// TODO(bounoable): Validate the stream config and return an error if it
		// doesn't match.
		return nil
	}

	if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("get stream info: %w [stream=%v]", err, streamName)
	}

	subjects := []string{subject}

	if _, err := jetstream.ctx.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: subjects,
	}); err != nil {
		return fmt.Errorf("add stream: %w [name=%v, subjects=%v]", err, streamName, subjects)
	}

	return nil
}

func (jetstream *jetStream) publish(ctx context.Context, bus *EventBus, evt event.Event) error {
	var buf bytes.Buffer
	if err := bus.enc.Encode(&buf, evt.Name(), evt.Data()); err != nil {
		return fmt.Errorf("encode event data: %w [event=%v, type(data)=%T]", err, evt.Name(), evt.Data())
	}

	b := buf.Bytes()

	env := envelope{
		ID:               evt.ID(),
		Name:             evt.Name(),
		Time:             evt.Time(),
		Data:             b,
		AggregateName:    evt.AggregateName(),
		AggregateID:      evt.AggregateID(),
		AggregateVersion: evt.AggregateVersion(),
	}

	buf = bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(env); err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}

	subject := bus.subjectFunc(env.Name)
	if _, err := jetstream.ctx.Publish(subject, buf.Bytes()); err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	return nil
}
