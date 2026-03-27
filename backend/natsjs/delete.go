package natsjs

import (
	"context"
	"fmt"

	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go/jetstream"
)

// Delete removes the given events from their per-aggregate-type streams and
// the KV index.
//
// Note: The sourced events stream may retain the deleted messages because
// JetStream source replication does not propagate deletes. This is an
// acceptable tradeoff — Find and aggregate-targeted Query will correctly
// exclude deleted events.
func (s *EventStore) Delete(ctx context.Context, events ...event.Event) error {
	if err := s.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	for _, evt := range events {
		if err := s.deleteOne(ctx, evt); err != nil {
			return err
		}
	}

	return nil
}

func (s *EventStore) deleteOne(ctx context.Context, evt event.Event) error {
	key := evt.ID().String()

	entry, err := s.kv.Get(ctx, key)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return fmt.Errorf("event not found: %s", key)
		}
		return fmt.Errorf("kv get: %w [event=%s]", err, key)
	}

	streamName, seq, err := parseKVIndex(entry.Value())
	if err != nil {
		return fmt.Errorf("parse index: %w [event=%s]", err, key)
	}

	stream, err := s.js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("get stream: %w [stream=%s, event=%s]", err, streamName, key)
	}

	if err := stream.DeleteMsg(ctx, seq); err != nil {
		return fmt.Errorf("delete message: %w [stream=%s, event=%s, seq=%d]", err, streamName, key, seq)
	}

	if err := s.kv.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete kv entry: %w [event=%s]", err, key)
	}

	return nil
}
