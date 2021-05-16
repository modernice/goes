// +build mongotag

package mongotags_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/modernice/goes/aggregate/tagging"
	"github.com/modernice/goes/aggregate/tagging/mongotags"
	"github.com/modernice/goes/aggregate/tagging/taggingtest"
	"github.com/modernice/goes/event"
)

func TestStore(t *testing.T) {
	taggingtest.Store(t, func(enc event.Encoder) tagging.Store {
		url := os.Getenv("MONGOTAG_URL")
		opts := []mongotags.Option{mongotags.URL(url)}
		if strings.HasPrefix(url, "mongodb+srv") {
			opts = append(opts, mongotags.Transactions(true))
		}
		store := mongotags.NewStore(enc, opts...)
		if _, err := store.Connect(context.Background()); err != nil {
			panic(err)
		}
		if err := store.Database().Drop(context.Background()); err != nil {
			panic(err)
		}
		return store
	})
}
