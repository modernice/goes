// +build mongostore

package mongostore_test

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/mongostore"
	"github.com/modernice/goes/event/eventstore/test"
)

func TestStore(t *testing.T) {
	var n int32
	test.EventStore(t, "mongostore", func(enc event.Encoder) event.Store {
		v := atomic.AddInt32(&n, 1)
		s := mongostore.New(enc, mongostore.Database(fmt.Sprintf("event_%d", v-1)))
		return s
	})
}
