//go:build postgres

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modernice/goes/persistence/model"
	"github.com/modernice/goes/backend/postgres"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
)

func TestEventStore(t *testing.T) {
	eventstoretest.Run(t, "postgres", func(enc codec.Encoding) event.Store {
		return postgres.NewEventStore(enc, postgres.URL(fmt.Sprintf("%s?pool_max_conn_lifetime=50ms", os.Getenv("POSTGRES_EVENTSTORE"))), postgres.Database(nextDatabase()))
	})
}

func TestConnect_NewEventStoreWithPoolOption(t *testing.T) {
	database := preCreateDatabase(t)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, fmt.Sprintf("%s/%s", os.Getenv("POSTGRES_EVENTSTORE"), database))
	if err != nil {
		t.Fatalf("Failed to create connection to database %q: %v", database, err)
	}
	defer pool.Close()

	store := postgres.NewEventStore(test.NewEncoder(), postgres.Pool(pool))
	if err := store.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect store: %v", err)
	}

	// Sanity check that the store is working
	_, err = store.Find(ctx, uuid.New())
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("Find returned unexpected error: %v", err)
	}
}

func TestConnect_ExistingTableWithCorrectSchema(t *testing.T) {
	database := preCreateDatabaseAndTable(t, `CREATE TABLE %s (
		id UUID PRIMARY KEY NOT NULL,
		name VARCHAR(255) NOT NULL,
		time BIGINT NOT NULL,
		aggregate_id UUID,
		aggregate_name VARCHAR(255),
		aggregate_version INTEGER,
		data JSONB
	)`)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, fmt.Sprintf("%s/%s", os.Getenv("POSTGRES_EVENTSTORE"), database))
	if err != nil {
		t.Fatalf("Failed to create connection to database %q: %v", database, err)
	}
	defer pool.Close()

	store := postgres.NewEventStore(test.NewEncoder(), postgres.Pool(pool))
	if err := store.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect store: %v", err)
	}
}

func TestConnect_ExistingTableWithInvalidSchema(t *testing.T) {
	database := preCreateDatabaseAndTable(t, `CREATE TABLE %s (
		id UUID PRIMARY KEY NOT NULL,
		name VARCHAR(255) NOT NULL,
		time BIGINT NOT NULL,
		aggregate_id BIGINT,
		aggregate_name VARCHAR(255),
		aggregate_version INTEGER
	)`)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, fmt.Sprintf("%s/%s", os.Getenv("POSTGRES_EVENTSTORE"), database))
	if err != nil {
		t.Fatalf("Failed to create connection to database %q: %v", database, err)
	}
	defer pool.Close()

	store := postgres.NewEventStore(test.NewEncoder(), postgres.Pool(pool))
	err = store.Connect(ctx)
	if err == nil {
		t.Fatal("Did not fail to Connect when existing table has invalid schema")
	}

	expectedError := `table "events" schema mismatch: missing columns: "data"; column "aggregate_id" should have type "uuid" but has type "bigint"`
	if err.Error() != expectedError {
		t.Fatalf("Connect returned unexpected error: %v", err)
	}
}

var databaseN uint64

func nextDatabase() string {
	n := atomic.AddUint64(&databaseN, 1)
	return fmt.Sprintf("goes_%d", n)
}

func preCreateDatabase(t *testing.T) string {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("POSTGRES_EVENTSTORE"))
	if err != nil {
		t.Fatalf("Failed to create bootstrapping connection to server: %v", err)
	}
	defer pool.Close()

	database := nextDatabase()

	_, err = pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", database))
	if err != nil {
		t.Fatalf("create %q database: %v", database, err)
	}

	return database
}

func preCreateDatabaseAndTable(t *testing.T, createTableFormatString string) string {
	database := preCreateDatabase(t)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, fmt.Sprintf("%s/%s", os.Getenv("POSTGRES_EVENTSTORE"), database))
	if err != nil {
		t.Fatalf("Failed to create bootstrapping connection to server: %v", err)
	}
	defer pool.Close()

	const tableName = "events"

	_, err = pool.Exec(ctx, fmt.Sprintf(createTableFormatString, tableName))
	if err != nil {
		t.Fatalf("create %q table in database %q: %v", tableName, database, err)
	}

	return database
}
