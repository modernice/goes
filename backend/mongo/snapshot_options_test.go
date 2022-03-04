//go:build mongo

package mongo

import (
	"context"
	"os"
	"testing"
)

func TestSnapshotDatabase(t *testing.T) {
	s := newSnapshotStore(SnapshotDatabase("foo"))
	if _, err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect shouldn't fail; failed with %q", err)
	}

	if s.db.Name() != "foo" {
		t.Errorf("database should have name %q; got %q", "foo", s.db.Name())
	}
}

func TestSnapshotCollection(t *testing.T) {
	s := newSnapshotStore(SnapshotCollection("foo"))
	if _, err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect shouldn't fail; failed with %q", err)
	}

	if s.col.Name() != "foo" {
		t.Errorf("collection should have name %q; got %q", "foo", s.db.Name())
	}
}

func newSnapshotStore(opts ...Option) *SnapshotStore {
	url := os.Getenv("MONGOSNAP_URL")
	opts = append([]Option{SnapshotURL(url)}, opts...)
	return NewSnapshotStore(opts...)
}
