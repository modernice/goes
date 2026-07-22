package eventstoreui

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStreamCatalogPagesFiltersAndMerges(t *testing.T) {
	base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	orderA := uuid.New()
	orderB := uuid.New()
	invoice := uuid.New()
	catalog := newStreamCatalog([]streamMetadata{
		{AggregateName: "order", AggregateID: orderA, CreatedAt: base},
		{AggregateName: "invoice", AggregateID: invoice, CreatedAt: base.Add(2 * time.Second)},
		{AggregateName: "order", AggregateID: orderB, CreatedAt: base.Add(time.Second)},
		{AggregateName: "order", AggregateID: orderA, CreatedAt: base},
	})

	if got := catalog.len(); got != 3 {
		t.Fatalf("expected three unique streams; got %d", got)
	}
	first, err := catalog.page(StreamFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].AggregateName != "invoice" || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", first)
	}
	second, err := catalog.page(StreamFilter{Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].AggregateID != orderA.String() {
		t.Fatalf("unexpected second page: %#v", second)
	}

	filtered, err := catalog.page(StreamFilter{AggregateNames: []string{"order"}, AggregateID: orderB.String(), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].AggregateID != orderB.String() {
		t.Fatalf("unexpected filtered page: %#v", filtered)
	}
	multiFiltered, err := catalog.page(StreamFilter{AggregateNames: []string{"missing", "invoice"}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(multiFiltered.Items) != 1 || multiFiltered.Items[0].AggregateName != "invoice" {
		t.Fatalf("unexpected multi-filtered page: %#v", multiFiltered)
	}

	newStream := streamMetadata{AggregateName: "order", AggregateID: uuid.New(), CreatedAt: base.Add(3 * time.Second)}
	if added := catalog.merge([]streamMetadata{newStream, newStream}); added != 1 {
		t.Fatalf("expected one merged stream; got %d", added)
	}
	latest, err := catalog.page(StreamFilter{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(latest.Items) != 1 || latest.Items[0].AggregateID != newStream.AggregateID.String() {
		t.Fatalf("unexpected latest stream: %#v", latest)
	}
}

func TestStreamCatalogRejectsInvalidAggregateID(t *testing.T) {
	_, err := newStreamCatalog(nil).page(StreamFilter{AggregateID: "not-a-uuid"})
	if err == nil {
		t.Fatal("expected invalid aggregate id error")
	}
}

type streamPollingReader struct {
	fakeReader
	streams []streamMetadata
}

func (reader *streamPollingReader) StreamStarts(context.Context, time.Time) ([]streamMetadata, error) {
	return reader.streams, nil
}

func TestStreamPollerMergesNewStreams(t *testing.T) {
	stream := streamMetadata{
		AggregateName: "order",
		AggregateID:   uuid.New(),
		CreatedAt:     time.Now().UTC(),
	}
	reader := &streamPollingReader{streams: []streamMetadata{stream}}
	catalog := newStreamCatalog(nil)
	app := &App{cfg: Config{StreamPollInterval: time.Millisecond}}
	ctx, cancel := context.WithCancel(context.Background())
	app.startStreamPoller(ctx, "orders", configuredStore{reader: reader, streams: catalog}, time.Now().UTC())
	defer func() {
		cancel()
		app.wg.Wait()
	}()

	deadline := time.Now().Add(time.Second)
	for catalog.len() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if catalog.len() != 1 {
		t.Fatal("expected poller to merge the new stream")
	}
}
