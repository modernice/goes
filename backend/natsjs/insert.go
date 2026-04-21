package natsjs

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/modernice/goes/event"
)

// ErrDuplicateEvent is returned when inserting an event with an ID that already
// exists in the store.
var ErrDuplicateEvent = errors.New("duplicate event")

// Insert inserts events into their respective per-aggregate-type JetStream
// streams and indexes them in the KV bucket. Each event's UUID is used as the
// NATS message ID for same-stream deduplication, and the KV index provides
// cross-stream deduplication.
func (s *EventStore) Insert(ctx context.Context, events ...event.Event) error {
	if err := s.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	for _, evt := range events {
		if err := s.insertOne(ctx, evt); err != nil {
			return err
		}
	}

	return nil
}

func (s *EventStore) insertOne(ctx context.Context, evt event.Event) error {
	aggID, aggName, _ := evt.Aggregate()

	if _, err := s.ensureAggStream(ctx, aggName); err != nil {
		return err
	}

	b, err := marshalEnvelope(s.enc, evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w [event=%s]", err, evt.Name())
	}

	subject := encodeSubject(s.namespace, evt.Name(), aggName, aggID)

	msg := &nats.Msg{
		Subject: subject,
		Data:    b,
		Header:  nats.Header{},
	}
	msg.Header.Set(nats.MsgIdHdr, evt.ID().String())

	ack, err := s.js.PublishMsg(ctx, msg)
	if err != nil {
		return fmt.Errorf("publish event: %w [event=%s, subject=%s]", err, evt.Name(), subject)
	}

	if ack.Duplicate {
		return fmt.Errorf("%w: %s", ErrDuplicateEvent, evt.ID())
	}

	// Use KV Create (not Put) for cross-stream duplicate detection. An event
	// with the same ID but different aggregate info would land in a different
	// stream, bypassing MsgId dedup. Create fails atomically if the key exists.
	streamName := s.aggStreamName(aggName)
	if _, err := s.kv.Create(ctx, evt.ID().String(), kvIndex(streamName, ack.Sequence)); err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			// Clean up the message we just published.
			if stream, ok := s.aggStreams.Load(streamName); ok {
				_ = stream.(jetstream.Stream).DeleteMsg(ctx, ack.Sequence)
			}
			return fmt.Errorf("%w: %s", ErrDuplicateEvent, evt.ID())
		}
		return fmt.Errorf("index event: %w [event=%s]", err, evt.Name())
	}

	return nil
}
