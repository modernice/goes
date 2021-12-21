package snapshot_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/internal/xtime"
)

func TestNew(t *testing.T) {
	now := xtime.Now()
	a := aggregate.New("foo", uuid.New(), aggregate.Version(8))
	snap, err := snapshot.New(a)
	if err != nil {
		t.Errorf("New shouldn't fail; failed with %q", err)
	}

	if snap.AggregateName() != a.AggregateName() {
		t.Errorf("AggregateName should return %q; got %q", a.AggregateID(), snap.AggregateName())
	}

	if snap.AggregateID() != a.AggregateID() {
		t.Errorf("AggregateID should return %q; got %q", a.AggregateID(), snap.AggregateID())
	}

	if snap.AggregateVersion() != a.AggregateVersion() {
		t.Errorf("AggregateVersion should return %d; got %d", a.AggregateVersion(), snap.AggregateVersion())
	}

	st := snap.Time()
	if st.UnixNano() < now.UnixNano() || st.UnixNano() > now.Add(50*time.Millisecond).UnixNano() {
		t.Errorf("Time should return ~%v; got %v", now, st)
	}

	if snap.State() != nil {
		t.Errorf("Data should return %v; got %v", nil, snap.State())
	}
}

func TestNew_marshaler(t *testing.T) {
	a := &mockSnapshot{Base: aggregate.New("foo", uuid.New())}
	snap, err := snapshot.New(a)
	if err != nil {
		t.Errorf("New shouldn't fail; failed with %q", err)
	}

	b, err := snapshot.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal shouldn't fail; failed with %q", err)
	}

	if !bytes.Equal(snap.State(), b) {
		t.Errorf("Data should return %v; got %v", b, snap.State())
	}
}

func TestTime(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	st := xtime.Now().Add(123456 * time.Millisecond)
	snap, err := snapshot.New(a, snapshot.Time(st))
	if err != nil {
		t.Fatalf("New shouldn't fail; failed with %q", err)
	}

	if !snap.Time().Equal(st) {
		t.Errorf("Time should return %v; got %v", st, snap.Time())
	}
}

func TestData(t *testing.T) {
	a := aggregate.New("foo", uuid.New())
	data := []byte{2, 4, 8}
	snap, err := snapshot.New(a, snapshot.Data(data))
	if err != nil {
		t.Fatalf("New shouldn't fail; failed with %q", err)
	}

	if !bytes.Equal(snap.State(), data) {
		t.Errorf("Data should return %v; got %v", data, snap.State())
	}
}
