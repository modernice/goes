//go:build postgres

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modernice/goes/backend/postgres"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/persistence/model"
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

const correctTableSQL = `CREATE TABLE %s (
	id UUID PRIMARY KEY NOT NULL,
	name VARCHAR(255) NOT NULL,
	time BIGINT NOT NULL,
	aggregate_id UUID,
	aggregate_name VARCHAR(255),
	aggregate_version INTEGER,
	data JSONB
)`

func TestConnect_ExistingTableWithMissingIndexes(t *testing.T) {
	database := preCreateDatabaseAndTable(t, correctTableSQL)

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

	// The connected role is allowed to create indexes, so Connect must have
	// created the missing ones.
	var unique bool
	if err := pool.QueryRow(ctx, "SELECT indisunique FROM pg_index WHERE indexrelid = 'goes_aggregate'::regclass").Scan(&unique); err != nil {
		t.Fatalf("goes_aggregate index was not created: %v", err)
	}
	if !unique {
		t.Fatal("goes_aggregate index is not unique")
	}
}

func TestConnect_ExistingTableWithConflictingIndex(t *testing.T) {
	database := preCreateDatabaseAndTable(t, correctTableSQL)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, fmt.Sprintf("%s/%s", os.Getenv("POSTGRES_EVENTSTORE"), database))
	if err != nil {
		t.Fatalf("Failed to create connection to database %q: %v", database, err)
	}
	defer pool.Close()

	// Occupy the name of the required unique index with an incompatible
	// definition. CREATE INDEX IF NOT EXISTS cannot fix this, so Connect must
	// fail.
	if _, err := pool.Exec(ctx, "CREATE INDEX goes_aggregate ON events (aggregate_id)"); err != nil {
		t.Fatalf("create conflicting index: %v", err)
	}

	store := postgres.NewEventStore(test.NewEncoder(), postgres.Pool(pool))
	err = store.Connect(ctx)
	if err == nil {
		t.Fatal("Did not fail to Connect when an existing index conflicts with the required definition")
	}

	if !strings.Contains(err.Error(), `"goes_aggregate"`) {
		t.Fatalf("Connect returned unexpected error: %v", err)
	}
}

func TestConnect_RestrictedUser(t *testing.T) {
	database := preCreateDatabaseAndTable(t, correctTableSQL)

	ctx := context.Background()

	admin, err := pgxpool.New(ctx, fmt.Sprintf("%s/%s", os.Getenv("POSTGRES_EVENTSTORE"), database))
	if err != nil {
		t.Fatalf("Failed to create connection to database %q: %v", database, err)
	}
	defer admin.Close()

	for _, stmt := range []string{
		"CREATE INDEX goes_name ON events (name)",
		"CREATE INDEX goes_time ON events (time)",
		"CREATE UNIQUE INDEX goes_aggregate ON events (aggregate_id, aggregate_name, aggregate_version)",
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	user := createRestrictedUser(t, admin, database)

	pool := connectAs(t, user, database)
	defer pool.Close()

	store := postgres.NewEventStore(test.NewEncoder(), postgres.Pool(pool))
	if err := store.Connect(ctx); err != nil {
		t.Fatalf("Connect must not require DDL privileges when the table and indexes exist: %v", err)
	}

	// Sanity check that the store is working
	_, err = store.Find(ctx, uuid.New())
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("Find returned unexpected error: %v", err)
	}
}

func TestConnect_RestrictedUserMissingUniqueIndex(t *testing.T) {
	database := preCreateDatabaseAndTable(t, correctTableSQL)

	ctx := context.Background()

	admin, err := pgxpool.New(ctx, fmt.Sprintf("%s/%s", os.Getenv("POSTGRES_EVENTSTORE"), database))
	if err != nil {
		t.Fatalf("Failed to create connection to database %q: %v", database, err)
	}
	defer admin.Close()

	user := createRestrictedUser(t, admin, database)

	pool := connectAs(t, user, database)
	defer pool.Close()

	store := postgres.NewEventStore(test.NewEncoder(), postgres.Pool(pool))
	err = store.Connect(ctx)
	if err == nil {
		t.Fatal("Did not fail to Connect when the unique index is missing and cannot be created")
	}

	if !strings.Contains(err.Error(), `"goes_aggregate"`) || !strings.Contains(err.Error(), "CREATE UNIQUE INDEX") {
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

// createRestrictedUser creates a login role that may read and write the events
// table but has no DDL privileges, mirroring an application that connects with
// lower permissions than the migration tooling that created the table.
func createRestrictedUser(t *testing.T, admin *pgxpool.Pool, database string) string {
	t.Helper()

	ctx := context.Background()

	user := database + "_app"
	for _, stmt := range []string{
		fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD 'restricted'", user),
		fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", database, user),
		fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", user),
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON events TO %s", user),
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	return user
}

func connectAs(t *testing.T, user, database string) *pgxpool.Pool {
	t.Helper()

	u, err := url.Parse(os.Getenv("POSTGRES_EVENTSTORE"))
	if err != nil {
		t.Fatalf("parse POSTGRES_EVENTSTORE: %v", err)
	}
	u.User = url.UserPassword(user, "restricted")
	u.Path = "/" + database

	pool, err := pgxpool.New(context.Background(), u.String())
	if err != nil {
		t.Fatalf("Failed to connect to database %q as %q: %v", database, user, err)
	}

	return pool
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
