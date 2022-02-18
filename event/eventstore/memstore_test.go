package eventstore_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
)

var _ event.Store[uuid.UUID] = eventstore.New[uuid.UUID]()

func TestMemstore(t *testing.T) {
	eventstoretest.Run(t, "memstore", func(codec.Encoding) event.Store[uuid.UUID] {
		return eventstore.New[uuid.UUID]()
	}, uuid.New)
}
