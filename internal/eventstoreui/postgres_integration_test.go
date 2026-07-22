package eventstoreui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresReaderIntegration(t *testing.T) {
	connectionURL := os.Getenv("GOES_UI_POSTGRES_TEST_URL")
	if connectionURL == "" {
		t.Skip("GOES_UI_POSTGRES_TEST_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, connectionURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	const table = "goes_ui_test_events"
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS goes_ui_test_events`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DROP TABLE IF EXISTS goes_ui_test_events`) })
	if _, err := pool.Exec(ctx, `CREATE TABLE goes_ui_test_events (
		id UUID PRIMARY KEY, name TEXT NOT NULL, time BIGINT NOT NULL,
		aggregate_id UUID, aggregate_name TEXT, aggregate_version INTEGER, data JSONB
	)`); err != nil {
		t.Fatal(err)
	}

	orderID, invoiceID := uuid.New(), uuid.New()
	baseTime := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	rows := []struct {
		id, aggregateID uuid.UUID
		name, aggregate string
		version         int
		at              time.Time
		data            string
	}{
		{uuid.New(), orderID, "order.created", "order", 1, baseTime, `{"number":"A-1"}`},
		{uuid.New(), orderID, "order.confirmed", "order", 2, baseTime.Add(time.Second), `{"by":"user"}`},
		{uuid.New(), invoiceID, "invoice.issued", "invoice", 1, baseTime.Add(2 * time.Second), `{"total":42}`},
	}
	for _, row := range rows {
		if _, err := pool.Exec(ctx, `INSERT INTO goes_ui_test_events
			(id, name, time, aggregate_id, aggregate_name, aggregate_version, data)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, row.id, row.name, row.at.UnixNano(), row.aggregateID, row.aggregate, row.version, row.data); err != nil {
			t.Fatal(err)
		}
	}

	reader, err := newPostgresReader(StoreConfig{URL: connectionURL, Table: table}, JSONDecoder{})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	assertReaderIntegration(t, ctx, reader, orderID.String(), rows[0].id.String())
}
