package indices

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestEventStoreCoreIndexesHaveNames(t *testing.T) {
	for _, index := range EventStoreCore() {
		var opts options.IndexOptions
		if index.Options == nil {
			t.Fatalf("core index %v has no name", index.Keys)
		}
		for _, set := range index.Options.List() {
			if err := set(&opts); err != nil {
				t.Fatalf("apply options for core index %v: %v", index.Keys, err)
			}
		}
		if opts.Name == nil || *opts.Name == "" {
			t.Fatalf("core index %v has no name", index.Keys)
		}
	}
}
