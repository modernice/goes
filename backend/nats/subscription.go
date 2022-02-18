package nats

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"

	"github.com/modernice/goes"
	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
)

type subscription[ID goes.ID] struct {
	event string

	sub  *nats.Subscription
	msgs chan []byte

	recipients []recipient[ID]

	subscribeQueue   chan subscribeJob[ID]
	unsubscribeQueue chan subscribeJob[ID]
	logQueue         chan logJob[ID]
	stop             chan struct{}
}

type recipient[ID goes.ID] struct {
	sub      *subscription[ID]
	events   chan event.Of[any, ID]
	errs     chan error
	unsubbed chan struct{}
}

type subscribeJob[ID goes.ID] struct {
	recipient recipient[ID]
	done      chan struct{}
}

type logJob[ID goes.ID] struct {
	err        error
	recipients []recipient[ID]
}

func newSubscription[ID goes.ID](
	event string,
	bus *EventBus[ID],
	sub *nats.Subscription,
	msgs chan []byte,
) *subscription[ID] {
	out := &subscription[ID]{
		event:            event,
		sub:              sub,
		msgs:             msgs,
		subscribeQueue:   make(chan subscribeJob[ID]),
		unsubscribeQueue: make(chan subscribeJob[ID]),
		logQueue:         make(chan logJob[ID]),
		stop:             bus.stop,
	}
	go out.work(bus)
	return out
}

func (sub *subscription[ID]) work(bus *EventBus[ID]) {
	defer sub.close()
	for {
		select {
		case <-sub.stop:
			return

		case job := <-sub.logQueue:
			rcpts := job.recipients
			if rcpts == nil {
				rcpts = sub.recipients
			}

			for _, rcpt := range rcpts {
				select {
				case <-sub.stop:
					return
				case <-rcpt.unsubbed:
				case rcpt.errs <- job.err:
				}
			}

		case subscribe := <-sub.subscribeQueue:
			sub.recipients = append(sub.recipients, subscribe.recipient)
			close(subscribe.done)

		case unsubscribe := <-sub.unsubscribeQueue:
			close(unsubscribe.recipient.errs)
			close(unsubscribe.recipient.events)
			for i, rcpt := range sub.recipients {
				if rcpt == unsubscribe.recipient {
					sub.recipients = append(sub.recipients[:i], sub.recipients[i+1:]...)
					break
				}
			}

			close(unsubscribe.done)

		case msg := <-sub.msgs:
			if err := sub.send(bus, msg); err != nil {
				go sub.err(err)
			}
		}
	}
}

// Print error message to ALL recipients in this subscription.
func (sub *subscription[ID]) err(err error) {
	select {
	case <-sub.stop:
	case sub.logQueue <- logJob[ID]{err: err}:
	}
}

func (sub *subscription[ID]) send(bus *EventBus[ID], msg []byte) error {
	var env envelope[ID]
	dec := gob.NewDecoder(bytes.NewReader(msg))
	if err := dec.Decode(&env); err != nil {
		return fmt.Errorf("gob decode envelope: %w", err)
	}

	data, err := bus.enc.Decode(bytes.NewReader(env.Data), env.Name)
	if err != nil {
		return fmt.Errorf("decode event data: %w [event=%v]", err, env.Name)
	}

	evt := event.New(
		env.ID,
		env.Name,
		data,
		event.Time(env.Time),
		event.Aggregate(
			env.AggregateID,
			env.AggregateName,
			env.AggregateVersion,
		),
	)

	for _, rcpt := range sub.recipients {
		select {
		case <-rcpt.sub.stop:
			return nil
		case rcpt.events <- evt:
		}
	}

	return nil
}

func (sub *subscription[ID]) subscribe(ctx context.Context) (recipient[ID], error) {
	done := make(chan struct{})

	rcpt := recipient[ID]{
		sub:      sub,
		events:   make(chan event.Of[any, ID]),
		errs:     make(chan error),
		unsubbed: make(chan struct{}),
	}

	select {
	case <-ctx.Done():
		return rcpt, ctx.Err()
	case sub.subscribeQueue <- subscribeJob[ID]{
		recipient: rcpt,
		done:      done,
	}:
	}

	select {
	case <-ctx.Done():
		return rcpt, ctx.Err()
	case <-done:
	}

	go func() {
		<-ctx.Done()
		close(rcpt.unsubbed)
		select {
		case <-sub.stop:
		case sub.unsubscribeQueue <- subscribeJob[ID]{
			recipient: rcpt,
			done:      make(chan struct{}),
		}:
		}
	}()

	return rcpt, nil
}

func (sub *subscription[ID]) close() {
	if err := sub.sub.Unsubscribe(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
		log.Printf(
			"[goes/backend/nats.subscription] Failed to unsubscribe from NATS: %v [event=%v, subject=%v]",
			err, sub.event, sub.sub.Subject,
		)
	}

	for _, rcpt := range sub.recipients {
		close(rcpt.errs)
		close(rcpt.events)
	}
}

func (rcpt recipient[ID]) log(err error) {
	select {
	case <-rcpt.sub.stop:
	case rcpt.sub.logQueue <- logJob[ID]{
		err:        err,
		recipients: []recipient[ID]{rcpt},
	}:
	}
}
