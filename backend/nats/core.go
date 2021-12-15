package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"

	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
)

const coreDriverName = "core"

type core struct{}

func Core() Driver {
	return core{}
}

func (core core) name() string { return coreDriverName }

func (core core) subscribe(ctx context.Context, bus *EventBus, event string) (*subscription, error) {
	msgs := make(chan []byte)
	errs := make(chan error)

	var sub *nats.Subscription
	var err error

	subject := bus.subjectFunc(event)
	if queue := bus.queueFunc(event); queue != "" {
		sub, err = bus.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
			defer msg.Ack()
			msgs <- msg.Data
		})
		if err != nil {
			return nil, fmt.Errorf("subscribe with queue group: %w [subject=%v queueGroup=%v]", err, subject, queue)
		}
	} else {
		sub, err = bus.conn.Subscribe(subject, func(msg *nats.Msg) {
			defer msg.Ack()
			msgs <- msg.Data
		})
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
		if err := sub.Drain(); err != nil {
			log.Printf("[nats.EventBus] drain subscription: %v [subject=%v, queue=%v]", err, sub.Subject, sub.Queue)
		}
	}()

	return &subscription{
		sub:       sub,
		msgs:      msgs,
		errs:      errs,
		unsubbbed: unsubbed,
	}, nil
}

func (core core) publish(ctx context.Context, bus *EventBus, evt event.Event) error {
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
	if err := bus.conn.Publish(subject, buf.Bytes()); err != nil {
		return fmt.Errorf("nats: %w", err)
	}

	return nil
}
