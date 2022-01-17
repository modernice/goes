package eventstore_test

import (
	"testing"

	"github.com/modernice/goes/backend/testing/eventstoretest"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
)

var _ event.Store[any] = eventstore.New[any]()

func TestMemstore(t *testing.T) {
	eventstoretest.Run(t, "memstore", func(codec.Encoding[any]) event.Store[any] {
		return eventstore.New[any]()
	})
}
