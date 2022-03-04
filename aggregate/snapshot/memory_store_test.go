package snapshot_test

import (
	"testing"

	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/storetest"
)

func TestStore(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		storetest.Run(t, snapshot.NewStore)
	})
}
