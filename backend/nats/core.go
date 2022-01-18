package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
)

// Core returns the NATS Core Driver (which is enabled by default):
//
//	bus := NewEventBus(enc, Use(Core())) // or
//	bus := NewEventBus(enc)
func Core() Driver {
	return &core{subs: make(map[string]*subscription)}
}

const coreDriverName = "core"

type core struct {
	sync.RWMutex

	subs map[string]*subscription
}

func (core *core) name() string { return coreDriverName }

func (core *core) subscribe(ctx context.Context, bus *EventBus, event string) (recipient, error) {
	core.Lock()
	defer core.Unlock()

	// If a subscription for that event already exists, return it.
	if sub, ok := core.subs[event]; ok {
		return sub.subscribe(ctx)
	}

	msgs := make(chan []byte)

	var nsub *nats.Subscription
	var err error

	subject := bus.subjectFunc(event)
	if queue := bus.queueFunc(event); queue != "" {
		nsub, err = bus.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
			msgs <- msg.Data
		})
		if err != nil {
			return recipient{}, fmt.Errorf("subscribe with queue group: %w [subject=%v queue=%v]", err, subject, queue)
		}
	} else {
		nsub, err = bus.conn.Subscribe(subject, func(msg *nats.Msg) { msgs <- msg.Data })
		if err != nil {
			return recipient{}, fmt.Errorf("subscribe: %w [subject=%v]", err, subject)
		}
	}

	sub := newSubscription(event, bus, nsub, msgs)
	core.subs[event] = sub

	rcpt, err := sub.subscribe(ctx)
	if err != nil {
		return rcpt, err
	}

	go func() {
		<-sub.stop
		core.Lock()
		defer core.Unlock()
		if csub, ok := core.subs[event]; ok && csub == sub {
			delete(core.subs, event)
		}
	}()

	return rcpt, nil
}

func (core *core) publish(ctx context.Context, bus *EventBus, evt event.EventOf[any]) error {
	var buf bytes.Buffer
	if err := bus.enc.Encode(&buf, evt.Name(), evt.Data()); err != nil {
		return fmt.Errorf("encode event data: %w [event=%v, type(data)=%T]", err, evt.Name(), evt.Data())
	}

	b := buf.Bytes()

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
