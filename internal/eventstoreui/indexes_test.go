package eventstoreui

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoStreamStartsIndexDefinition(t *testing.T) {
	index := mongoIndexDefinition{
		Name:                    mongoStreamStartsIndex,
		Keys:                    bson.D{{Key: "timeNano", Value: int32(1)}},
		PartialFilterExpression: bson.D{{Key: "aggregateVersion", Value: int32(1)}},
	}
	if !isMongoStreamStartsIndex(index) {
		t.Fatal("expected the required MongoDB index definition to match")
	}

	index.PartialFilterExpression[0].Value = int32(2)
	if isMongoStreamStartsIndex(index) {
		t.Fatal("expected an incompatible MongoDB partial filter to be rejected")
	}
}

func TestMongoEventTimeIndexDefinition(t *testing.T) {
	for _, direction := range []int32{-1, 1} {
		index := mongoIndexDefinition{
			Name: mongoEventTimeIndex,
			Keys: bson.D{{Key: "timeNano", Value: direction}},
		}
		if !isMongoEventTimeIndex(index) {
			t.Fatalf("expected direction %d to provide the event-time index", direction)
		}
	}

	partial := mongoIndexDefinition{
		Name:                    mongoEventTimeIndex,
		Keys:                    bson.D{{Key: "timeNano", Value: int32(1)}},
		PartialFilterExpression: bson.D{{Key: "aggregateVersion", Value: int32(1)}},
	}
	if isMongoEventTimeIndex(partial) {
		t.Fatal("a partial index cannot serve all event metadata")
	}
}

func TestPostgresStreamStartsIndexName(t *testing.T) {
	if got := postgresStreamStartsIndex("events"); got != "goes_ui_events_stream_starts_v1" {
		t.Fatalf("unexpected default index name %q", got)
	}

	longTable := "schema." + strings.Repeat("a", 64)
	got := postgresStreamStartsIndex(longTable)
	if len(got) > 63 {
		t.Fatalf("index name exceeds PostgreSQL's identifier limit: %q", got)
	}
	if got != postgresStreamStartsIndex(longTable) {
		t.Fatal("expected a stable PostgreSQL index name")
	}
	if got == postgresStreamStartsIndex("schema."+strings.Repeat("a", 63)+"b") {
		t.Fatal("expected long table names to retain a unique hash suffix")
	}
	if postgresStreamStartsIndexLock("events") != postgresStreamStartsIndexLock("events") {
		t.Fatal("expected a stable PostgreSQL setup lock")
	}
	if postgresStreamStartsIndexLock("events") == postgresStreamStartsIndexLock("other_events") {
		t.Fatal("expected PostgreSQL setup locks to be table-specific")
	}
}
