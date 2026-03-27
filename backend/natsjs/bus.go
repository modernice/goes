package natsjs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go/jetstream"
)

// Publish publishes events by inserting them into the JetStream stream.
// Subscribers will receive the events through their aggregate stream consumers.
func (s *EventStore) Publish(ctx context.Context, events ...event.Event) error {
	return s.Insert(ctx, events...)
}

// Subscribe subscribes to events by name. Consumers are created on all known
// per-aggregate-type streams, filtered by event name. New aggregate streams
// created after subscribing are detected and subscribed to automatically.
// Subscribe to "*" to receive all events.
func (s *EventStore) Subscribe(ctx context.Context, names ...string) (<-chan event.Event, <-chan error, error) {
	if err := s.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}

	evtCh := make(chan event.Event)
	errCh := make(chan error)

	sub := &subscription{
		store:     s,
		ctx:       ctx,
		names:     names,
		evtCh:     evtCh,
		errCh:     errCh,
		streams:   make(map[string]struct{}),
		startTime: time.Now(),
	}

	// Subscribe to all currently known aggregate streams.
	s.aggStreams.Range(func(key, value any) bool {
		streamName := key.(string)
		stream := value.(jetstream.Stream)
		if err := sub.addStream(streamName, stream); err != nil {
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
		}
		return true
	})

	// Watch for new aggregate streams being created.
	go sub.watchNewStreams()

	// Close channels when context is done.
	go func() {
		<-ctx.Done()
		sub.stop()
	}()

	return evtCh, errCh, nil
}

// subscription manages consumers across multiple aggregate streams for a
// single Subscribe call.
type subscription struct {
	store     *EventStore
	ctx       context.Context
	names     []string
	evtCh     chan event.Event
	errCh     chan error
	startTime time.Time

	mu      sync.Mutex
	streams map[string]struct{} // tracks which streams we're subscribed to
	cctxs   []jetstream.ConsumeContext
	closed  bool
}

func (sub *subscription) addStream(streamName string, stream jetstream.Stream) error {
	sub.mu.Lock()
	defer sub.mu.Unlock()

	if sub.closed {
		return nil
	}
	if _, ok := sub.streams[streamName]; ok {
		return nil
	}

	info := stream.CachedInfo()
	if info == nil || len(info.Config.Subjects) == 0 {
		return nil
	}

	for _, name := range sub.names {
		subject := subscribeSubjectForAggStream(info.Config.Subjects[0], name)

		cfg := jetstream.ConsumerConfig{
			FilterSubject: subject,
			DeliverPolicy: jetstream.DeliverByStartTimePolicy,
			OptStartTime:  &sub.startTime,
			AckPolicy:     jetstream.AckExplicitPolicy,
		}

		if sub.store.serviceName != "" {
			cfg.Durable = consumerName(sub.store.serviceName, streamName, subject)
		}

		cons, err := sub.store.js.CreateOrUpdateConsumer(sub.ctx, streamName, cfg)
		if err != nil {
			return fmt.Errorf("create consumer: %w [stream=%s, subject=%s]", err, streamName, subject)
		}

		cctx, err := cons.Consume(func(msg jetstream.Msg) {
			evt, err := unmarshalEnvelope(sub.store.enc, msg.Data())
			if err != nil {
				select {
				case sub.errCh <- fmt.Errorf("unmarshal event: %w", err):
				case <-sub.ctx.Done():
				}
				return
			}

			msg.Ack()

			select {
			case sub.evtCh <- evt:
			case <-sub.ctx.Done():
			}
		})
		if err != nil {
			return fmt.Errorf("start consume: %w [stream=%s]", err, streamName)
		}

		sub.cctxs = append(sub.cctxs, cctx)
	}

	sub.streams[streamName] = struct{}{}
	return nil
}

// watchNewStreams waits for new aggregate streams to be created and subscribes
// to them.
func (sub *subscription) watchNewStreams() {
	for {
		// Get the current notification channel.
		sub.store.newAggStreamMu.Lock()
		ch := sub.store.newAggStream
		sub.store.newAggStreamMu.Unlock()

		select {
		case <-sub.ctx.Done():
			return
		case <-ch:
			// Channel was closed — a new aggregate stream was created.
			sub.store.aggStreams.Range(func(key, value any) bool {
				streamName := key.(string)
				sub.mu.Lock()
				_, known := sub.streams[streamName]
				sub.mu.Unlock()
				if known {
					return true
				}

				stream := value.(jetstream.Stream)
				if err := sub.addStream(streamName, stream); err != nil {
					select {
					case sub.errCh <- err:
					case <-sub.ctx.Done():
					}
				}
				return true
			})
		}
	}
}

func (sub *subscription) stop() {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	sub.closed = true
	for _, cctx := range sub.cctxs {
		cctx.Stop()
	}
	close(sub.evtCh)
	close(sub.errCh)
}

// consumerName builds a deterministic durable consumer name from the service
// name, stream name, and subject filter. The name is stable across restarts so
// that multiple instances of the same service share the consumer.
func consumerName(serviceName, streamName, subject string) string {
	h := sha256.Sum256([]byte(streamName + "\x00" + subject))
	return serviceName + "_" + hex.EncodeToString(h[:8])
}

// subscribeSubjectForAggStream builds a subscribe subject filter for an event
// name within an aggregate stream. The streamSubject is the stream's own
// subject pattern (e.g., "goes.orders.>"). We replace the ">" with the event
// name filter.
//
// Example: streamSubject="goes.orders.>", eventName="order.placed"
// Result: "goes.orders.*.order_placed"
func subscribeSubjectForAggStream(streamSubject, eventName string) string {
	// Strip the trailing ".>"
	base := streamSubject[:len(streamSubject)-1]
	if eventName == "*" {
		return base + ">"
	}
	// base is "goes.orders." — add wildcard for aggID + event name
	return base + "*." + escapeSubjectToken(eventName)
}
