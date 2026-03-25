//go:build postgres

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modernice/goes/backend/postgres"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

func TestEventStore(t *testing.T) {
	eventstoretest.Run(t, "postgres", func(enc codec.Encoding) event.Store {
		time.Sleep(60 * time.Millisecond) // Wait for previous connections to close
		return postgres.NewEventStore(enc, postgres.URL(fmt.Sprintf("%s?pool_max_conn_lifetime=50ms", os.Getenv("POSTGRES_EVENTSTORE"))), postgres.Database(nextDatabase()))
	})
}

func TestEventStoreWithPool(t *testing.T) {
	eventstoretest.Run(t, "postgres", func(enc codec.Encoding) event.Store {
		ctx := context.Background()

		pool, err := pgxpool.New(ctx, os.Getenv("POSTGRES_EVENTSTORE"))
		if err != nil {
			t.Fatalf("Failed to create bootstrapping connection to server: %v", err)
		}

		database := nextDatabase()
		_, err = pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", database))
		if err != nil {
			pool.Close()
			t.Fatalf("create %q database: %v", database, err)
		}
		pool.Close()

		time.Sleep(60 * time.Millisecond) // Wait for previous connections to close
		pool, err = pgxpool.New(ctx, fmt.Sprintf("%s/%s?pool_max_conn_lifetime=50ms", os.Getenv("POSTGRES_EVENTSTORE"), database))
		if err != nil {
			t.Fatalf("Failed to create connection to database %q: %v", database, err)
		}

		return postgres.NewEventStore(enc, postgres.Pool(pool))
	})
}

var databaseN uint64

func nextDatabase() string {
	n := atomic.AddUint64(&databaseN, 1)
	return fmt.Sprintf("goes_%d", n)
}
