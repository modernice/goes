package eventstore_test

import (
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/eventstore/test"
)

var _ event.Store = eventstore.New()

func TestMemstore(t *testing.T) {
	test.EventStore(t, "memstore", func(event.Encoder) event.Store {
		return eventstore.New()
	})
}
