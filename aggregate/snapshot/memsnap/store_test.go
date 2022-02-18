package memsnap_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/snapshot/memsnap"
	"github.com/modernice/goes/aggregate/snapshot/storetest"
)

func TestStore(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		storetest.Run(t, memsnap.New[uuid.UUID], uuid.New)
	})
}
