package natsjs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go/jetstream"
)

// Find retrieves an event by its UUID. It uses the KV index to look up the
// stream name and sequence number, then fetches the raw message from the
// appropriate per-aggregate-type stream.
func (s *EventStore) Find(ctx context.Context, id uuid.UUID) (event.Event, error) {
	if err := s.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	entry, err := s.kv.Get(ctx, id.String())
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return nil, fmt.Errorf("event not found: %s", id)
		}
		return nil, fmt.Errorf("kv get: %w", err)
	}

	streamName, seq, err := parseKVIndex(entry.Value())
	if err != nil {
		return nil, fmt.Errorf("parse index: %w [event=%s]", err, id)
	}

	stream, err := s.js.Stream(ctx, streamName)
	if err != nil {
		return nil, fmt.Errorf("get stream: %w [stream=%s]", err, streamName)
	}

	msg, err := stream.GetMsg(ctx, seq)
	if err != nil {
		return nil, fmt.Errorf("get message: %w [stream=%s, seq=%d]", err, streamName, seq)
	}

	evt, err := unmarshalEnvelope(s.enc, msg.Data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal event: %w [stream=%s, seq=%d]", err, streamName, seq)
	}

	return evt, nil
}
