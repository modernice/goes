package natsjs

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/nats-io/nats.go/jetstream"
)

const fetchBatchSize = 256

// queryTarget describes which stream and subject filter to use for a query.
type queryTarget struct {
	streamName string
	subject    string
}

// Query searches for events in the store matching the given query.
//
// Queries with aggregate filters (name, ID, or references) are routed to the
// appropriate per-aggregate-type stream for efficient scanning. Queries without
// aggregate filters scan all known aggregate streams.
//
// Fine-grained filtering (time, version, event IDs) is applied in-memory via
// query.Test after subject-level pre-filtering.
func (s *EventStore) Query(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	if err := s.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}

	targets := s.queryTargets(q)
	// When querying multiple streams, we must always buffer and sort because
	// each stream's events are ordered independently.
	needsSort := needsBufferedSort(q) || len(targets) > 1

	out := make(chan event.Event)
	errs := make(chan error)

	go func() {
		defer close(out)
		defer close(errs)

		var collected []event.Event

		for _, target := range targets {
			// Stream may not exist yet (e.g., querying before any inserts).
			_, streamErr := s.js.Stream(ctx, target.streamName)
			if streamErr != nil {
				continue // no stream = no events to return
			}

			cons, err := s.js.OrderedConsumer(ctx, target.streamName, jetstream.OrderedConsumerConfig{
				FilterSubjects: []string{target.subject},
				DeliverPolicy:  jetstream.DeliverAllPolicy,
			})
			if err != nil {
				select {
				case <-ctx.Done():
				case errs <- fmt.Errorf("create consumer: %w [stream=%s, subject=%s]", err, target.streamName, target.subject):
				}
				return
			}

			if err := s.drainConsumer(ctx, cons, q, needsSort, &collected, out, errs); err != nil {
				if !errors.Is(err, ctx.Err()) {
					select {
					case <-ctx.Done():
					case errs <- err:
					}
				}
				return
			}
		}

		if needsSort && len(collected) > 0 {
			sortings := q.Sortings()
			if len(sortings) == 0 {
				// Default to time ascending when merging across multiple streams.
				sortings = []event.SortOptions{{Sort: event.SortTime, Dir: event.SortAsc}}
			}
			sorted := event.SortMulti(collected, sortings...)
			for _, evt := range sorted {
				select {
				case <-ctx.Done():
					return
				case out <- evt:
				}
			}
		}
	}()

	return out, errs, nil
}

// drainConsumer reads all available messages from an ordered consumer.
func (s *EventStore) drainConsumer(
	ctx context.Context,
	cons jetstream.Consumer,
	q event.Query,
	buffer bool,
	collected *[]event.Event,
	out chan<- event.Event,
	errs chan<- error,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batch, err := cons.FetchNoWait(fetchBatchSize)
		if err != nil {
			if isConsumerDone(err) {
				return nil
			}
			return fmt.Errorf("fetch: %w", err)
		}

		count := 0
		for msg := range batch.Messages() {
			count++
			evt, err := unmarshalEnvelope(s.enc, msg.Data())
			if err != nil {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case errs <- fmt.Errorf("unmarshal event: %w", err):
				}
				continue
			}

			if !query.Test(q, evt) {
				continue
			}

			if buffer {
				*collected = append(*collected, evt)
			} else {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- evt:
				}
			}
		}

		if err := batch.Error(); err != nil {
			if isConsumerDone(err) {
				return nil
			}
			return fmt.Errorf("batch error: %w", err)
		}

		// If we got fewer messages than requested, we've drained the stream.
		if count < fetchBatchSize {
			return nil
		}
	}
}

// queryTargets determines which stream(s) and subject filter(s) to use.
func (s *EventStore) queryTargets(q event.Query) []queryTarget {
	// Specific aggregate references (name + ID pairs) — most selective.
	// Route to per-aggregate-type streams.
	if refs := q.Aggregates(); len(refs) > 0 {
		targets := make([]queryTarget, 0, len(refs))
		for _, ref := range refs {
			targets = append(targets, queryTarget{
				streamName: s.aggStreamName(ref.Name),
				subject:    aggregateFilterSubject(s.namespace,ref.Name, ref.ID),
			})
		}
		return targets
	}

	// Aggregate name + optional ID filters — route to per-aggregate-type streams.
	if aggNames := q.AggregateNames(); len(aggNames) > 0 {
		aggIDs := q.AggregateIDs()

		if len(aggIDs) > 0 && len(aggNames) == 1 {
			// Single aggregate type with specific IDs.
			targets := make([]queryTarget, 0, len(aggIDs))
			for _, id := range aggIDs {
				targets = append(targets, queryTarget{
					streamName: s.aggStreamName(aggNames[0]),
					subject:    aggregateFilterSubject(s.namespace,aggNames[0], id),
				})
			}
			return targets
		}

		// Multiple aggregate types, or single type without ID filter.
		targets := make([]queryTarget, 0, len(aggNames))
		for _, name := range aggNames {
			targets = append(targets, queryTarget{
				streamName: s.aggStreamName(name),
				subject:    aggregateFilterSubject(s.namespace,name, uuid.Nil),
			})
		}
		return targets
	}

	// No aggregate filter — query all known aggregate streams.
	return s.allAggStreamTargets(q)
}

// isConsumerDone returns true if the error indicates the consumer has no more
// messages to deliver (deleted, closed, or not found).
func isConsumerDone(err error) bool {
	return errors.Is(err, jetstream.ErrMsgIteratorClosed) ||
		errors.Is(err, jetstream.ErrConsumerDeleted) ||
		errors.Is(err, jetstream.ErrConsumerNotFound)
}

// allAggStreamTargets returns query targets for all known aggregate streams.
// Event name and other filters are applied in-memory via query.Test.
func (s *EventStore) allAggStreamTargets(_ event.Query) []queryTarget {
	var targets []queryTarget
	s.aggStreams.Range(func(key, value any) bool {
		streamName := key.(string)
		stream := value.(jetstream.Stream)
		info := stream.CachedInfo()
		// Use the stream's own subject as the filter — guaranteed to match.
		subject := info.Config.Subjects[0]
		targets = append(targets, queryTarget{
			streamName: streamName,
			subject:    subject,
		})
		return true
	})
	return targets
}

// needsBufferedSort returns true if the query requires sorting that doesn't
// match the natural stream order (publish time ascending).
//
// Within a single aggregate's stream, publish order == version order, so
// SortAggregateVersion ASC also matches the natural order when the query
// targets a single aggregate.
func needsBufferedSort(q event.Query) bool {
	sortings := q.Sortings()
	if len(sortings) == 0 {
		return false
	}

	if len(sortings) == 1 && sortings[0].Dir == event.SortAsc {
		switch sortings[0].Sort {
		case event.SortTime:
			// Time ascending == natural stream order.
			return false
		case event.SortAggregateVersion:
			// Version ascending == natural stream order when targeting a single
			// aggregate (events are published sequentially).
			if isSingleAggregateQuery(q) {
				return false
			}
		}
	}

	return true
}

// isSingleAggregateQuery returns true if the query targets exactly one
// aggregate instance (name + ID).
func isSingleAggregateQuery(q event.Query) bool {
	if refs := q.Aggregates(); len(refs) == 1 && refs[0].ID != uuid.Nil {
		return true
	}
	if names := q.AggregateNames(); len(names) == 1 {
		if ids := q.AggregateIDs(); len(ids) == 1 {
			return true
		}
	}
	return false
}
