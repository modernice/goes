package memtags_test

import (
	"testing"

	"github.com/modernice/goes/aggregate/tagging"
	"github.com/modernice/goes/aggregate/tagging/memtags"
	"github.com/modernice/goes/aggregate/tagging/taggingtest"
	"github.com/modernice/goes/event"
)

func TestStore(t *testing.T) {
	taggingtest.Store(t, func(event.Encoder) tagging.Store {
		return memtags.NewStore()
	})
}
