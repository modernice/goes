package eventstore_test

import (
	"testing"

	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
)

var _ event.Store = eventstore.New()

func TestMemstore(t *testing.T) {
	eventstoretest.Run(t, "memstore", func(codec.Encoding[any]) event.Store {
		return eventstore.New()
	})
}
