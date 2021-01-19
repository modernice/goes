// +build mongostore

package mongostore_test

import (
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/mongostore/mongotest"
	"github.com/modernice/goes/event/eventstore/test"
)

func TestStore(t *testing.T) {
	test.EventStore(t, "mongostore", func(enc event.Encoder) event.Store {
		return mongotest.NewStore(enc)
	})
}
