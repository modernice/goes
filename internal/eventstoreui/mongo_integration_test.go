package eventstoreui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestMongoReaderIntegration(t *testing.T) {
	connectionURL := os.Getenv("GOES_UI_MONGO_TEST_URL")
	if connectionURL == "" {
		t.Skip("GOES_UI_MONGO_TEST_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(connectionURL))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	collection := client.Database("goes_ui_test").Collection("events")
	_ = collection.Drop(ctx)
	t.Cleanup(func() { _ = collection.Drop(context.Background()) })

	orderID, invoiceID := uuid.New(), uuid.New()
	firstID := uuid.New()
	baseTime := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	documents := []any{
		mongoEvent{ID: firstID, Name: "order.created", Time: baseTime, TimeNano: baseTime.UnixNano(), AggregateName: "order", AggregateID: orderID, AggregateVersion: 1, Data: []byte(`{"number":"A-1"}`)},
		mongoEvent{ID: uuid.New(), Name: "order.confirmed", Time: baseTime.Add(time.Second), TimeNano: baseTime.Add(time.Second).UnixNano(), AggregateName: "order", AggregateID: orderID, AggregateVersion: 2, Data: []byte(`{"by":"user"}`)},
		mongoEvent{ID: uuid.New(), Name: "invoice.issued", Time: baseTime.Add(2 * time.Second), TimeNano: baseTime.Add(2 * time.Second).UnixNano(), AggregateName: "invoice", AggregateID: invoiceID, AggregateVersion: 1, Data: []byte(`{"total":42}`)},
	}
	if _, err := collection.InsertMany(ctx, documents); err != nil {
		t.Fatal(err)
	}

	reader, err := newMongoReader(StoreConfig{URL: connectionURL, Database: "goes_ui_test", Collection: "events"}, JSONDecoder{})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	assertReaderIntegration(t, ctx, reader, orderID.String(), firstID.String())
}
