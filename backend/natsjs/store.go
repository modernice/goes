// Package natsjs provides a combined event store and event bus backed by NATS
// JetStream.
//
// Events are stored in per-aggregate-type JetStream streams for efficient
// aggregate fetching.
//
// Subject format: <namespace>.<aggregateName>.<aggregateID>.<eventName>
//
// A JetStream KV bucket is used as an index for Find-by-UUID lookups.
package natsjs

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var (
	// DefaultNamespace is the default namespace for subjects and stream names.
	DefaultNamespace = "goes"

	// DefaultKVBucket is the default KV bucket name for the event ID index.
	DefaultKVBucket = "goes_idx"
)

// Ensure EventStore implements both event.Store and event.Bus.
var (
	_ event.Store = (*EventStore)(nil)
	_ event.Bus   = (*EventStore)(nil)
)

// EventStore is a combined event store and event bus backed by NATS JetStream.
//
// Events are stored in per-aggregate-type streams for fast aggregate fetching.
type EventStore struct {
	enc codec.Encoding

	url      string
	conn     *nats.Conn
	natsOpts []nats.Option

	namespace   string
	kvBucket    string
	serviceName string

	js jetstream.JetStream
	kv jetstream.KeyValue

	// aggStreams caches per-aggregate-type streams. Key is the stream name.
	aggStreams sync.Map // map[string]jetstream.Stream
	// aggStreamsMu protects creation of new aggregate streams.
	aggStreamsMu sync.Mutex
	// newAggStream is closed when a new aggregate stream is created,
	// then replaced with a fresh channel. All watchers blocked on the
	// old channel wake up simultaneously.
	newAggStream   chan struct{}
	newAggStreamMu sync.Mutex

	onceConnect sync.Once
}

// Option configures an EventStore.
type Option func(*EventStore)

// NewEventStore returns a new JetStream-backed event store and event bus.
func NewEventStore(enc codec.Encoding, opts ...Option) *EventStore {
	if enc == nil {
		enc = event.NewRegistry()
	}

	s := &EventStore{
		enc:              enc,
		namespace: DefaultNamespace,
		kvBucket:  DefaultKVBucket,
		newAggStream:     make(chan struct{}, 1),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// URL sets the NATS server URL. If not set, the NATS_URL environment variable
// is used, falling back to nats.DefaultURL.
func URL(url string) Option {
	return func(s *EventStore) { s.url = url }
}

// Conn provides an existing NATS connection to the event store.
func Conn(conn *nats.Conn) Option {
	return func(s *EventStore) { s.conn = conn }
}

// NATSOpts appends NATS connection options.
func NATSOpts(opts ...nats.Option) Option {
	return func(s *EventStore) { s.natsOpts = append(s.natsOpts, opts...) }
}

// Namespace sets the namespace used for NATS subjects and JetStream stream
// names. Default is "goes".
func Namespace(ns string) Option {
	return func(s *EventStore) { s.namespace = ns }
}

// KVBucket sets the KV bucket name for the event ID index.
// Default is "goes_idx".
func KVBucket(name string) Option {
	return func(s *EventStore) { s.kvBucket = name }
}

// LoadBalancer enables load-balanced event subscriptions across instances of
// the same service. When set, subscribers sharing the same service name will
// use durable JetStream consumers with a deterministic name, causing NATS to
// distribute messages so that only one instance receives each event.
//
// Without this option, every subscriber instance receives every event.
func LoadBalancer(serviceName string) Option {
	return func(s *EventStore) { s.serviceName = serviceName }
}

// Connection returns the underlying NATS connection.
func (s *EventStore) Connection() *nats.Conn {
	return s.conn
}

// Connect establishes the connection to NATS and initializes JetStream
// resources (KV bucket). Per-aggregate-type streams are created lazily on
// first insert.
//
// Connect is called automatically by Insert, Find, Query, Delete, Publish,
// and Subscribe if not called explicitly.
func (s *EventStore) Connect(ctx context.Context) error {
	var err error
	s.onceConnect.Do(func() {
		if s.conn == nil {
			url := s.url
			if url == "" {
				url = os.Getenv("NATS_URL")
			}
			if url == "" {
				url = nats.DefaultURL
			}

			s.conn, err = nats.Connect(url, s.natsOpts...)
			if err != nil {
				err = fmt.Errorf("connect to nats: %w", err)
				return
			}
		}

		s.js, err = jetstream.New(s.conn)
		if err != nil {
			err = fmt.Errorf("create jetstream context: %w", err)
			return
		}

		s.kv, err = s.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
			Bucket: s.kvBucket,
		})
		if err != nil {
			err = fmt.Errorf("create/update kv bucket: %w", err)
			return
		}
	})
	return err
}

func (s *EventStore) cleanupForTests(ctx context.Context) error {
	if s.js == nil {
		return nil
	}

	// Delete all aggregate streams.
	s.aggStreams.Range(func(key, _ any) bool {
		_ = s.js.DeleteStream(ctx, key.(string))
		return true
	})

	// Delete the KV bucket.
	_ = s.js.DeleteKeyValue(ctx, s.kvBucket)

	return nil
}

// Disconnect closes the underlying NATS connection.
func (s *EventStore) Disconnect(ctx context.Context) error {
	if s.conn == nil {
		return nil
	}

	closed := make(chan struct{})
	s.conn.SetClosedHandler(func(*nats.Conn) { close(closed) })
	s.conn.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-closed:
		return nil
	}
}

// aggStreamName returns the JetStream stream name for an aggregate type.
func (s *EventStore) aggStreamName(aggName string) string {
	if aggName == "" {
		aggName = subjectSentinel
	}
	return s.namespace + "_agg_" + escapeSubjectToken(aggName)
}

// ensureAggStream ensures a per-aggregate-type stream exists and returns it.
func (s *EventStore) ensureAggStream(ctx context.Context, aggName string) (jetstream.Stream, error) {
	streamName := s.aggStreamName(aggName)

	// Fast path: already cached.
	if v, ok := s.aggStreams.Load(streamName); ok {
		return v.(jetstream.Stream), nil
	}

	s.aggStreamsMu.Lock()
	defer s.aggStreamsMu.Unlock()

	// Double-check after acquiring lock.
	if v, ok := s.aggStreams.Load(streamName); ok {
		return v.(jetstream.Stream), nil
	}

	var subjects []string
	if aggName == "" {
		subjects = noAggStreamSubjects(s.namespace)
	} else {
		subjects = aggStreamSubjects(s.namespace, aggName)
	}

	stream, err := s.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: subjects,
	})
	if err != nil {
		return nil, fmt.Errorf("create aggregate stream: %w [aggregate=%s]", err, aggName)
	}

	s.aggStreams.Store(streamName, stream)

	// Notify all subscriptions about the new stream by closing the
	// current channel and replacing it with a fresh one.
	s.newAggStreamMu.Lock()
	close(s.newAggStream)
	s.newAggStream = make(chan struct{})
	s.newAggStreamMu.Unlock()

	return stream, nil
}

// kvIndex encodes the stream name and sequence number for KV storage.
// Format: "<streamName>:<sequence>"
func kvIndex(streamName string, seq uint64) []byte {
	return []byte(streamName + ":" + strconv.FormatUint(seq, 10))
}

// parseKVIndex decodes a KV value into a stream name and sequence number.
func parseKVIndex(data []byte) (streamName string, seq uint64, err error) {
	s := string(data)
	idx := strings.LastIndexByte(s, ':')
	if idx < 0 {
		return "", 0, fmt.Errorf("invalid kv index: %q", s)
	}
	streamName = s[:idx]
	seq, err = strconv.ParseUint(s[idx+1:], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("parse sequence: %w", err)
	}
	return streamName, seq, nil
}
