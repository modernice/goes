package cleanup_test

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/cleanup"
	"github.com/modernice/goes/aggregate/snapshot/memsnap"
	"github.com/modernice/goes/aggregate/snapshot/query"
)

type mockAggregate struct {
	aggregate.Aggregate
	mockState
}

type mockState struct {
	A bool
	B int
	C string
}

func TestService(t *testing.T) {
	store := memsnap.New()

	foo1 := &mockAggregate{Aggregate: aggregate.New("foo", uuid.New())}
	foo2 := &mockAggregate{Aggregate: aggregate.New("foo", uuid.New())}

	snap1, err := snapshot.New(foo1)
	if err != nil {
		t.Fatalf("make Snapshot: %v", err)
	}

	snap2, err := snapshot.New(foo2, snapshot.Time(time.Now().Add(-2*time.Hour)))
	if err != nil {
		t.Fatalf("make Snapshot: %v", err)
	}

	if err := store.Save(context.Background(), snap1); err != nil {
		t.Fatalf("Save shouldn't fail; failed with %q", err)
	}
	if err := store.Save(context.Background(), snap2); err != nil {
		t.Fatalf("Save shouldn't fail; failed with %q", err)
	}

	c := cleanup.NewService(10*time.Millisecond, time.Hour)

	errs, err := c.Start(store)
	if err != nil {
		t.Fatalf("Start shouldn't fail; failed with %q", err)
	}

	select {
	case err, ok := <-errs:
		if !ok {
			t.Fatalf("error channel shouldn't be closed!")
		}
		t.Fatal(err)
	case <-time.After(100 * time.Millisecond):
	}

	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop shouldn't fail; failed with %q", err)
	}

	res, errs, err := store.Query(context.Background(), query.New())
	if err != nil {
		t.Fatalf("Query shouldn't fail; failed with %q", err)
	}

	snaps, err := snapshot.Drain(context.Background(), res, errs)
	if err != nil {
		t.Fatalf("Drain shouldn't fail; failed with %q", err)
	}

	if len(snaps) != 1 {
		t.Fatalf("Query should return exactly 1 Snapshot; got %d", len(snaps))
	}

	snap := snaps[0]
	if snap.AggregateID() != snap1.AggregateID() || snap.AggregateVersion() != snap1.AggregateVersion() {
		t.Errorf("Query returned the wrong Snapshot. want=%v got=%v", snap1, snap)
	}
}

func TestService_Start_errStarted(t *testing.T) {
	svc := cleanup.NewService(time.Minute, time.Minute)
	store := memsnap.New()

	if _, err := svc.Start(store); err != nil {
		t.Fatalf("Start shouldn't fail; failed with %q", err)
	}

	if _, err := svc.Start(store); !errors.Is(err, cleanup.ErrStarted) {
		t.Fatalf("Start should fail with %q; got %q", cleanup.ErrStarted, err)
	}
}

func TestService_Start_nilStore(t *testing.T) {
	svc := cleanup.NewService(time.Minute, time.Minute)
	if _, err := svc.Start(nil); err == nil {
		t.Fatalf("Start should fail without a Store!")
	}
}

func TestService_Stop(t *testing.T) {
	svc := cleanup.NewService(time.Minute, time.Minute)
	store := memsnap.New()

	if _, err := svc.Start(store); err != nil {
		t.Fatalf("Start shouldn't fail; failed with %q", err)
	}

	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop shouldn't fail; failed with %q", err)
	}

	if _, err := svc.Start(store); err != nil {
		t.Fatalf("Start shouldn't fail; failed with %q", err)
	}
}

func TestService_Stop_errNotStarted(t *testing.T) {
	svc := cleanup.NewService(time.Minute, time.Minute)

	if err := svc.Stop(context.Background()); !errors.Is(err, cleanup.ErrStopped) {
		t.Fatalf("Stop shouldn't fail; failed with %q", err)
	}
}

func (a *mockAggregate) MarshalSnapshot() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(a.mockState); err != nil {
		return nil, fmt.Errorf("gob: %w", err)
	}
	return buf.Bytes(), nil
}

func (a *mockAggregate) UnmarshalSnapshot(p []byte) error {
	if err := gob.NewDecoder(bytes.NewReader(p)).Decode(&a.mockState); err != nil {
		return fmt.Errorf("gob: %w", err)
	}
	return nil
}
