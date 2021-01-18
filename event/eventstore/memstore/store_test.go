package memstore_test

import (
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/memstore"
	"github.com/modernice/goes/event/eventstore/test"
)

var _ event.Store = &memstore.Store{}

func TestStore(t *testing.T) {
	test.EventStore(t, func() event.Store {
		return memstore.New()
	})
}
