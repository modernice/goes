package eventstoreui

import (
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
)

type streamKey struct {
	name string
	id   uuid.UUID
}

type streamCatalog struct {
	mu      sync.RWMutex
	streams []streamMetadata
	known   map[streamKey]struct{}
	names   map[string]string
}

func newStreamCatalog(streams []streamMetadata) *streamCatalog {
	catalog := &streamCatalog{}
	catalog.replace(streams)
	return catalog
}

func (catalog *streamCatalog) replace(streams []streamMetadata) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()

	catalog.streams = make([]streamMetadata, 0, len(streams))
	catalog.known = make(map[streamKey]struct{}, len(streams))
	catalog.names = make(map[string]string)
	catalog.add(streams)
	catalog.sort()
}

func (catalog *streamCatalog) merge(streams []streamMetadata) int {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()

	added := catalog.add(streams)
	if added > 0 {
		catalog.sort()
	}
	return added
}

func (catalog *streamCatalog) add(streams []streamMetadata) int {
	added := 0
	for _, stream := range streams {
		if stream.AggregateName == "" || stream.AggregateID == uuid.Nil {
			continue
		}
		name, ok := catalog.names[stream.AggregateName]
		if !ok {
			name = stream.AggregateName
			catalog.names[name] = name
		}
		stream.AggregateName = name
		stream.CreatedAt = stream.CreatedAt.UTC()
		key := streamKey{name: name, id: stream.AggregateID}
		if _, ok := catalog.known[key]; ok {
			continue
		}
		catalog.known[key] = struct{}{}
		catalog.streams = append(catalog.streams, stream)
		added++
	}
	return added
}

func (catalog *streamCatalog) sort() {
	sort.Slice(catalog.streams, func(i, j int) bool {
		return catalog.streams[i].CreatedAt.After(catalog.streams[j].CreatedAt)
	})
}

func (catalog *streamCatalog) len() int {
	catalog.mu.RLock()
	defer catalog.mu.RUnlock()
	return len(catalog.streams)
}

func (catalog *streamCatalog) page(filter StreamFilter) (StreamPage, error) {
	limit := normalizedLimit(filter.Limit)
	aggregateNames := make(map[string]struct{}, len(filter.AggregateNames))
	for _, name := range filter.AggregateNames {
		aggregateNames[name] = struct{}{}
	}
	var aggregateID uuid.UUID
	if filter.AggregateID != "" {
		parsed, err := uuid.Parse(filter.AggregateID)
		if err != nil {
			return StreamPage{}, fmt.Errorf("invalid aggregate id: %w", err)
		}
		aggregateID = parsed
	}
	var cursor streamCursor
	if err := decodeCursor(filter.Cursor, &cursor); err != nil {
		return StreamPage{}, err
	}

	catalog.mu.RLock()
	defer catalog.mu.RUnlock()

	items := make([]Stream, 0, limit+1)
	for _, stream := range catalog.streams {
		if cursor.TimeNano != 0 && stream.CreatedAt.UnixNano() >= cursor.TimeNano {
			continue
		}
		if len(aggregateNames) > 0 {
			if _, ok := aggregateNames[stream.AggregateName]; !ok {
				continue
			}
		}
		if aggregateID != uuid.Nil && stream.AggregateID != aggregateID {
			continue
		}
		items = append(items, Stream{
			AggregateName: stream.AggregateName,
			AggregateID:   stream.AggregateID.String(),
			CreatedAt:     stream.CreatedAt,
		})
		if len(items) > limit {
			break
		}
	}

	page := StreamPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		page.Items = items[:limit]
		page.NextCursor = encodeCursor(streamCursor{TimeNano: last.CreatedAt.UnixNano()})
	}
	return page, nil
}
