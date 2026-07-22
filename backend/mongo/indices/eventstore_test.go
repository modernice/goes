package indices

import "testing"

func TestEventStoreCoreIndexesHaveNames(t *testing.T) {
	for _, index := range EventStoreCore() {
		if index.Options == nil || index.Options.Name == nil || *index.Options.Name == "" {
			t.Fatalf("core index %v has no name", index.Keys)
		}
	}
}
