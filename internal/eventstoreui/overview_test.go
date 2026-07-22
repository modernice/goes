package eventstoreui

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOverviewProjectionSeedsMergesAndSnapshots(t *testing.T) {
	base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	projection := newOverviewProjection(
		Summary{TotalEvents: 3, LatestEventTime: timePointer(base)},
		Facets{
			EventNames:     []Facet{{Value: "order.created", Count: 2}, {Value: "invoice.issued", Count: 1}},
			AggregateNames: []Facet{{Value: "order", Count: 2}, {Value: "invoice", Count: 1}},
		},
	)

	existingID := uuid.New()
	projection.seed([]eventMetadata{{
		ID: existingID, Name: "order.created", AggregateName: "order", Time: base,
	}}, base.Add(-time.Minute))

	newID := uuid.New()
	added := projection.merge([]eventMetadata{
		{ID: existingID, Name: "order.created", AggregateName: "order", Time: base},
		{ID: newID, Name: "order.confirmed", AggregateName: "order", Time: base.Add(time.Second)},
	}, base.Add(-time.Minute))
	if added != 1 {
		t.Fatalf("expected one new event; got %d", added)
	}

	summary, facets := projection.snapshot(2)
	if summary.TotalEvents != 4 || summary.EventTypes != 3 || summary.AggregateTypes != 2 || summary.AggregateStreams != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.LatestEventTime == nil || !summary.LatestEventTime.Equal(base.Add(time.Second)) {
		t.Fatalf("unexpected latest event time: %#v", summary.LatestEventTime)
	}
	if len(facets.EventNames) != 3 || facets.EventNames[0] != (Facet{Value: "order.created", Count: 2}) {
		t.Fatalf("unexpected event facets: %#v", facets.EventNames)
	}
	if len(facets.AggregateNames) != 2 || facets.AggregateNames[0] != (Facet{Value: "order", Count: 3}) {
		t.Fatalf("unexpected aggregate facets: %#v", facets.AggregateNames)
	}
}

func TestSortedFacetsLimitsAndOrdersValues(t *testing.T) {
	got := sortedFacets(map[string]int64{"z": 1, "a": 1, "popular": 5}, 2)
	want := []Facet{{Value: "popular", Count: 5}, {Value: "a", Count: 1}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected facets: %#v", got)
	}
}

type overviewPollingReader struct {
	fakeReader
	events []eventMetadata
}

func (reader *overviewPollingReader) EventMetadata(context.Context, time.Time) ([]eventMetadata, error) {
	return reader.events, nil
}

func TestOverviewPollerMergesNewEvents(t *testing.T) {
	reader := &overviewPollingReader{events: []eventMetadata{{
		ID: uuid.New(), Name: "order.created", AggregateName: "order", Time: time.Now().UTC(),
	}}}
	projection := newOverviewProjection(Summary{}, Facets{})
	app := &App{cfg: Config{StreamPollInterval: time.Millisecond}}
	ctx, cancel := context.WithCancel(context.Background())
	app.startOverviewPoller(ctx, "orders", configuredStore{reader: reader, overview: projection}, time.Now().UTC())
	defer func() {
		cancel()
		app.wg.Wait()
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		summary, _ := projection.snapshot(0)
		if summary.TotalEvents > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	summary, _ := projection.snapshot(0)
	if summary.TotalEvents != 1 || summary.EventTypes != 1 || summary.AggregateTypes != 1 {
		t.Fatalf("unexpected polled summary: %#v", summary)
	}
}

func timePointer(value time.Time) *time.Time { return &value }
