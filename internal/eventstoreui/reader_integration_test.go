package eventstoreui

import (
	"context"
	"testing"
	"time"
)

func assertReaderIntegration(t *testing.T, ctx context.Context, reader Reader, orderID, firstEventID string) {
	t.Helper()
	if err := reader.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	// Index setup must be safe to repeat on every application startup.
	if err := reader.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	if err := reader.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	summary, err := reader.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalEvents != 3 || summary.EventTypes != 3 || summary.AggregateTypes != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	facets, err := reader.Facets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(facets.EventNames) != 3 || len(facets.AggregateNames) != 2 {
		t.Fatalf("unexpected facets: %#v", facets)
	}
	metadata, err := reader.EventMetadata(ctx, time.Date(2026, 7, 22, 11, 59, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 3 || metadata[0].ID.String() != firstEventID {
		t.Fatalf("unexpected event metadata: %#v", metadata)
	}

	firstPage, err := reader.Events(ctx, EventFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Items) != 2 || firstPage.NextCursor == "" || firstPage.Items[0].Name != "invoice.issued" {
		t.Fatalf("unexpected first event page: %#v", firstPage)
	}
	secondPage, err := reader.Events(ctx, EventFilter{Limit: 2, Cursor: firstPage.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].Name != "order.created" {
		t.Fatalf("unexpected second event page: %#v", secondPage)
	}

	event, err := reader.Event(ctx, firstEventID)
	if err != nil {
		t.Fatal(err)
	}
	if event.Name != "order.created" || event.DataDecodeError != "" {
		t.Fatalf("unexpected event: %#v", event)
	}

	streams, err := reader.StreamStarts(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 2 {
		t.Fatalf("unexpected stream starts: %#v", streams)
	}

	streamEvents, err := reader.StreamEvents(ctx, "order", orderID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(streamEvents.Items) != 2 || streamEvents.Items[0].Aggregate.Version != 1 || streamEvents.Items[1].Aggregate.Version != 2 {
		t.Fatalf("unexpected stream events: %#v", streamEvents)
	}
}
