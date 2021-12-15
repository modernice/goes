//go:build mongosnap

package mongosnap

import (
	"context"
	"os"
	"testing"
)

func TestDatabase(t *testing.T) {
	s := newStore(Database("foo"))
	if _, err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect shouldn't fail; failed with %q", err)
	}

	if s.db.Name() != "foo" {
		t.Errorf("database should have name %q; got %q", "foo", s.db.Name())
	}
}

func TestCollection(t *testing.T) {
	s := newStore(Collection("foo"))
	if _, err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect shouldn't fail; failed with %q", err)
	}

	if s.col.Name() != "foo" {
		t.Errorf("collection should have name %q; got %q", "foo", s.db.Name())
	}
}

func newStore(opts ...Option) *Store {
	url := os.Getenv("MONGOSNAP_URL")
	opts = append([]Option{URL(url)}, opts...)
	return New(opts...)
}
