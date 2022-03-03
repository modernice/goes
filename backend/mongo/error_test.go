package mongo_test

import (
	"testing"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/backend/mongo"
)

func TestVersionError_IsInconsistencyError(t *testing.T) {
	var versionError mongo.VersionError
	if got := aggregate.IsConsistencyError(versionError); !got {
		t.Fatalf("aggregate.IsConsistencyError() should return %v for a mongo.VersionError; got %v", true, got)
	}

	var cmdError mongo.CommandError
	if got := aggregate.IsConsistencyError(cmdError); !got {
		t.Fatalf("aggregate.IsConsistencyError() should return %v for a mongo.CommandError; got %v", true, got)
	}
}
