package eventstoreui

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type overviewProjection struct {
	mu              sync.RWMutex
	totalEvents     int64
	latestEventTime *time.Time
	eventNames      map[string]int64
	aggregateNames  map[string]int64
	seen            map[uuid.UUID]time.Time
}

func newOverviewProjection(summary Summary, facets Facets) *overviewProjection {
	projection := &overviewProjection{
		totalEvents:    summary.TotalEvents,
		eventNames:     facetCounts(facets.EventNames),
		aggregateNames: facetCounts(facets.AggregateNames),
		seen:           make(map[uuid.UUID]time.Time),
	}
	if summary.LatestEventTime != nil {
		latest := summary.LatestEventTime.UTC()
		projection.latestEventTime = &latest
	}
	return projection
}

func facetCounts(facets []Facet) map[string]int64 {
	counts := make(map[string]int64, len(facets))
	for _, facet := range facets {
		if facet.Value != "" {
			counts[facet.Value] = facet.Count
		}
	}
	return counts
}

func (projection *overviewProjection) seed(events []eventMetadata, cutoff time.Time) {
	projection.mu.Lock()
	defer projection.mu.Unlock()
	projection.remember(events)
	projection.prune(cutoff)
}

func (projection *overviewProjection) merge(events []eventMetadata, cutoff time.Time) int {
	projection.mu.Lock()
	defer projection.mu.Unlock()

	added := 0
	for _, event := range events {
		if event.ID == uuid.Nil {
			continue
		}
		if _, ok := projection.seen[event.ID]; ok {
			continue
		}
		at := event.Time.UTC()
		projection.seen[event.ID] = at
		projection.totalEvents++
		if event.Name != "" {
			projection.eventNames[event.Name]++
		}
		if event.AggregateName != "" {
			projection.aggregateNames[event.AggregateName]++
		}
		if projection.latestEventTime == nil || at.After(*projection.latestEventTime) {
			latest := at
			projection.latestEventTime = &latest
		}
		added++
	}
	projection.prune(cutoff)
	return added
}

func (projection *overviewProjection) remember(events []eventMetadata) {
	for _, event := range events {
		if event.ID != uuid.Nil {
			projection.seen[event.ID] = event.Time.UTC()
		}
	}
}

func (projection *overviewProjection) prune(cutoff time.Time) {
	for id, at := range projection.seen {
		if at.Before(cutoff) {
			delete(projection.seen, id)
		}
	}
}

func (projection *overviewProjection) snapshot(aggregateStreams int64) (Summary, Facets) {
	projection.mu.RLock()
	defer projection.mu.RUnlock()

	summary := Summary{
		TotalEvents:      projection.totalEvents,
		EventTypes:       int64(len(projection.eventNames)),
		AggregateTypes:   int64(len(projection.aggregateNames)),
		AggregateStreams: aggregateStreams,
	}
	if projection.latestEventTime != nil {
		latest := *projection.latestEventTime
		summary.LatestEventTime = &latest
	}
	return summary, Facets{
		EventNames:     sortedFacets(projection.eventNames, maxPageSize),
		AggregateNames: sortedFacets(projection.aggregateNames, maxPageSize),
	}
}

func sortedFacets(counts map[string]int64, limit int) []Facet {
	facets := make([]Facet, 0, len(counts))
	for value, count := range counts {
		facets = append(facets, Facet{Value: value, Count: count})
	}
	sort.Slice(facets, func(i, j int) bool {
		if facets[i].Count == facets[j].Count {
			return facets[i].Value < facets[j].Value
		}
		return facets[i].Count > facets[j].Count
	})
	if len(facets) > limit {
		facets = facets[:limit]
	}
	return facets
}
