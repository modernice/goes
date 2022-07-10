//go:build postgres

package postgres_test

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/modernice/goes/backend/postgres"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

func TestEventStore(t *testing.T) {
	eventstoretest.Run(t, "postgres", func(enc codec.Encoding) event.Store {
		return postgres.NewEventStore(enc, postgres.Database(nextDatabase()))
	})
}

var databaseN uint64

func nextDatabase() string {
	n := atomic.AddUint64(&databaseN, 1)
	return fmt.Sprintf("goes_%d", n)
}
