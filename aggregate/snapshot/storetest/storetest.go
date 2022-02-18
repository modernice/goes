package storetest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	stdtime "time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/aggregate/snapshot/query"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/xaggregate"
	"github.com/modernice/goes/internal/xtime"
)

// StoreFactory creates Stores.
type StoreFactory[ID goes.ID] func() snapshot.Store[ID]

// Run runs the Store tests.
func Run[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	run(t, "Save", testSave[ID], newStore, newID)
	run(t, "Latest", testLatest[ID], newStore, newID)
	run(t, "Latest (multiple available)", testLatestMultipleAvailable[ID], newStore, newID)
	run(t, "Latest (not found)", testLatestNotFound[ID], newStore, newID)
	run(t, "Version", testVersion[ID], newStore, newID)
	run(t, "Version (not found)", testVersionNotFound[ID], newStore, newID)
	run(t, "Limit", testLimit[ID], newStore, newID)
	run(t, "Query", testQuery[ID], newStore, newID)
	run(t, "Delete", testDelete[ID], newStore, newID)
}

func run[ID goes.ID](t *testing.T, name string, runner func(*testing.T, StoreFactory[ID], func() ID), newStore StoreFactory[ID], newID func() ID) {
	t.Run(name, func(t *testing.T) {
		runner(t, newStore, newID)
	})
}

func testSave[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	a := &snapshotter[ID]{
		Base:  aggregate.New("foo", newID()),
		state: state{Foo: 3},
	}

	snap, err := snapshot.New[ID](a)
	if err != nil {
		t.Fatalf("Marshal shouldn't fail; failed with %q", err)
	}

	if err := s.Save(context.Background(), snap); err != nil {
		t.Errorf("Save shouldn't fail; failed with %q", err)
	}
}

func testLatest[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	a := &snapshotter[ID]{
		Base:  aggregate.New("foo", newID()),
		state: state{Foo: 3},
	}

	snap, err := snapshot.New[ID](a)
	if err != nil {
		t.Fatalf("Marshal shouldn't fail; failed with %q", err)
	}

	if err := s.Save(context.Background(), snap); err != nil {
		t.Errorf("Save shouldn't fail; failed with %q", err)
	}

	latest, err := s.Latest(context.Background(), a.AggregateName(), a.AggregateID())
	if err != nil {
		t.Fatalf("Latest shouldn't fail; failed with %q", err)
	}

	if snap.AggregateName() != latest.AggregateName() {
		t.Errorf("AggregateName should return %q; got %q", snap.AggregateName(), latest.AggregateName())
	}

	if snap.AggregateID() != latest.AggregateID() {
		t.Errorf("AggregateID should return %q; got %q", snap.AggregateID(), latest.AggregateID())
	}

	if snap.AggregateVersion() != latest.AggregateVersion() {
		t.Errorf("AggregateVersion should return %q; got %q", snap.AggregateVersion(), latest.AggregateVersion())
	}

	wantTime := snap.Time()
	if !latest.Time().Equal(wantTime) {
		t.Errorf("Time should return %v; got %v", wantTime, latest.Time())
	}

	if !bytes.Equal(snap.State(), latest.State()) {
		t.Errorf("Data should return %v; got %v", snap.State(), latest.State())
	}
}

func testLatestMultipleAvailable[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	id := newID()
	a10 := &snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(10))}
	a20 := &snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(20))}
	snap10, _ := snapshot.New[ID](a10)
	snap20, _ := snapshot.New[ID](a20)

	if err := s.Save(context.Background(), snap20); err != nil {
		t.Errorf("Save shouldn't fail; failed with %q", err)
	}

	if err := s.Save(context.Background(), snap10); err != nil {
		t.Errorf("Save shouldn't fail; failed with %q", err)
	}

	latest, err := s.Latest(context.Background(), "foo", id)
	if err != nil {
		t.Fatalf("Latest shouldn't fail; failed with %q", err)
	}

	if latest.AggregateName() != "foo" {
		t.Errorf("AggregateName should return %q; got %q", "foo", latest.AggregateName())
	}

	if latest.AggregateID() != id {
		t.Errorf("AggregateID should return %q; got %q", id, latest.AggregateID())
	}

	if latest.AggregateVersion() != 20 {
		t.Errorf("AggregateVersion should return %q; got %q", 20, latest.AggregateVersion())
	}

	wantTime := snap20.Time()
	if !latest.Time().Equal(wantTime) {
		t.Errorf("Time should return %v; got %v", wantTime, latest.Time())
	}

	if !bytes.Equal(snap20.State(), latest.State()) {
		t.Errorf("Data should return %v; got %v", snap20.State(), latest.State())
	}
}

func testLatestNotFound[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	snap, err := s.Latest(context.Background(), "foo", newID())
	if snap != nil {
		t.Errorf("Latest should return no Snapshot; got %v", snap)
	}

	if err == nil {
		t.Errorf("Latest should fail; got %q", err)
	}
}

func testVersion[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	id := newID()
	a10 := &snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(10))}
	a20 := &snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(20))}
	snap10, _ := snapshot.New[ID](a10)
	snap20, _ := snapshot.New[ID](a20)

	if err := s.Save(context.Background(), snap10); err != nil {
		t.Fatalf("failed to save Snapshot: %v", err)
	}

	if err := s.Save(context.Background(), snap20); err != nil {
		t.Fatalf("failed to save Snapshot: %v", err)
	}

	snap, err := s.Version(context.Background(), "foo", id, 10)
	if err != nil {
		t.Fatalf("Version shouldn't fail; failed with %q", err)
	}

	if snap.AggregateVersion() != snap10.AggregateVersion() {
		t.Errorf(
			"Version should return Snapshot with version %d; got version %d",
			snap10.AggregateVersion(),
			snap.AggregateVersion(),
		)
	}
}

func testVersionNotFound[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()

	snap, err := s.Version(context.Background(), "foo", newID(), 10)
	if snap != nil {
		t.Errorf("Version should return no Snapshot; got %v", snap)
	}

	if err == nil {
		t.Errorf("Version should fail; got %q", err)
	}
}

func testLimit[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	run(t, "Basic", testLimitBasic[ID], newStore, newID)
	run(t, "NotFound", testLimitNotFound[ID], newStore, newID)
}

func testLimitBasic[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()

	id := newID()
	as := []aggregate.AggregateOf[ID]{
		&snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(1))},
		&snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(5))},
		&snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(10))},
		&snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(20))},
	}
	snaps := makeSnaps(as)

	for _, snap := range snaps {
		if err := s.Save(context.Background(), snap); err != nil {
			t.Fatalf("Save shouldn't fail; failed with %q", err)
		}
	}

	snap, err := s.Limit(context.Background(), "foo", id, 19)
	if err != nil {
		t.Fatalf("Limit shouldn't fail; failed with %q", err)
	}

	if snap.AggregateVersion() != 10 {
		t.Errorf("Limit should return the Snapshot with version %d; got version %d", 10, snap.AggregateVersion())
	}
}

func testLimitNotFound[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()

	id := newID()
	as := []aggregate.AggregateOf[ID]{
		&snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(10))},
		&snapshotter[ID]{Base: aggregate.New("foo", id, aggregate.Version(20))},
	}
	snaps := makeSnaps(as)

	for _, snap := range snaps {
		if err := s.Save(context.Background(), snap); err != nil {
			t.Fatalf("Save shouldn't fail; failed with %q", err)
		}
	}

	snap, err := s.Limit(context.Background(), "foo", id, 9)
	if err == nil {
		t.Errorf("Limit should fail!")
	}

	if snap != nil {
		t.Errorf("Limit should return no Snapshot; got %v", snap)
	}
}

func testQuery[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	run(t, "Name", testQueryName[ID], newStore, newID)
	run(t, "ID", testQueryID[ID], newStore, newID)
	run(t, "Version", testQueryVersion[ID], newStore, newID)
	run(t, "Time", testQueryTime[ID], newStore, newID)
	run(t, "Sorting", testQuerySorting[ID], newStore, newID)
}

func testQueryName[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	foos, _ := xaggregate.Make(newID, 5, xaggregate.Name("foo"))
	bars, _ := xaggregate.Make(newID, 5, xaggregate.Name("bar"))

	for i, foo := range foos {
		id, name, _ := foo.Aggregate()
		foos[i] = &snapshotter[ID]{Base: aggregate.New(name, id)}
	}

	for i, bar := range bars {
		id, name, _ := bar.Aggregate()
		bars[i] = &snapshotter[ID]{Base: aggregate.New(name, id)}
	}

	fooSnaps := makeSnaps(foos)
	barSnaps := makeSnaps(bars)
	snaps := append(fooSnaps, barSnaps...)

	for _, snap := range snaps {
		if err := s.Save(context.Background(), snap); err != nil {
			t.Fatalf("Save shouldn't fail; failed with %q", err)
		}
	}

	result, err := runQuery[ID](s, query.New[ID](query.Name("foo")))
	if err != nil {
		t.Fatal(err)
	}

	assertSame(t, fooSnaps, result)
}

func testQueryID[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	as, _ := xaggregate.Make(newID, 5, xaggregate.Name("foo"))

	for i, a := range as {
		id, name, _ := a.Aggregate()
		as[i] = &snapshotter[ID]{Base: aggregate.New(name, id)}
	}

	snaps := makeSnaps(as)

	for _, snap := range snaps {
		if err := s.Save(context.Background(), snap); err != nil {
			t.Fatalf("Save shouldn't fail; failed with %q", err)
		}
	}

	result, err := runQuery[ID](s, query.New[ID](query.ID(
		pick.AggregateID[ID](as[0]),
		pick.AggregateID[ID](as[4]),
	)))
	if err != nil {
		t.Fatal(err)
	}

	assertSame(t, []snapshot.Snapshot[ID]{
		snaps[0],
		snaps[4],
	}, result)
}

func testQueryVersion[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	as := []aggregate.AggregateOf[ID]{
		&snapshotter[ID]{Base: aggregate.New("foo", newID(), aggregate.Version(1))},
		&snapshotter[ID]{Base: aggregate.New("foo", newID(), aggregate.Version(5))},
		&snapshotter[ID]{Base: aggregate.New("foo", newID(), aggregate.Version(10))},
	}
	snaps := makeSnaps(as)

	for _, snap := range snaps {
		if err := s.Save(context.Background(), snap); err != nil {
			t.Fatalf("Save shouldn't fail; failed with %q", err)
		}
	}

	result, err := runQuery[ID](s, query.New[ID](
		query.Version(version.Exact(1, 10)),
	))
	if err != nil {
		t.Fatal(err)
	}

	assertSame(t, []snapshot.Snapshot[ID]{
		snaps[0],
		snaps[2],
	}, result)
}

func testQueryTime[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	as := []aggregate.AggregateOf[ID]{
		&snapshotter[ID]{Base: aggregate.New("foo", newID())},
		&snapshotter[ID]{Base: aggregate.New("foo", newID())},
		&snapshotter[ID]{Base: aggregate.New("foo", newID())},
	}
	snaps := make([]snapshot.Snapshot[ID], len(as))
	for i := range as {
		var err error
		var opts []snapshot.Option
		if i == 2 {
			opts = append(opts, snapshot.Time(xtime.Now().Add(-stdtime.Minute)))
		}
		if snaps[i], err = snapshot.New(as[i], opts...); err != nil {
			t.Fatalf("failed to make Snapshot: %v", err)
		}
	}

	for _, snap := range snaps {
		if err := s.Save(context.Background(), snap); err != nil {
			t.Fatalf("Save shouldn't fail; failed with %q", err)
		}
	}

	result, err := runQuery[ID](s, query.New[ID](
		query.Time(time.After(xtime.Now().Add(-stdtime.Second))),
	))
	if err != nil {
		t.Fatal(err)
	}

	assertSame(t, snaps[:2], result)
}

func testQuerySorting[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	ids := make([]ID, 9)
	for i := range ids {
		ids[i] = newID()
	}
	sort.Slice(ids, func(a, b int) bool {
		return ids[a].String() < ids[b].String()
	})
	as := []aggregate.AggregateOf[ID]{
		&snapshotter[ID]{Base: aggregate.New("bar1", ids[0], aggregate.Version(1))},
		&snapshotter[ID]{Base: aggregate.New("bar2", ids[1], aggregate.Version(2))},
		&snapshotter[ID]{Base: aggregate.New("bar3", ids[2], aggregate.Version(3))},
		&snapshotter[ID]{Base: aggregate.New("baz1", ids[3], aggregate.Version(4))},
		&snapshotter[ID]{Base: aggregate.New("baz2", ids[4], aggregate.Version(5))},
		&snapshotter[ID]{Base: aggregate.New("baz3", ids[5], aggregate.Version(6))},
		&snapshotter[ID]{Base: aggregate.New("foo1", ids[6], aggregate.Version(7))},
		&snapshotter[ID]{Base: aggregate.New("foo2", ids[7], aggregate.Version(8))},
		&snapshotter[ID]{Base: aggregate.New("foo3", ids[8], aggregate.Version(9))},
	}
	snaps := makeSnaps(as)

	tests := []struct {
		name string
		q    query.Query[ID]
		want []snapshot.Snapshot[ID]
	}{
		{
			name: "SortAggregateName(asc)",
			q:    query.New[ID](query.SortBy(aggregate.SortName, aggregate.SortAsc)),
			want: snaps,
		},
		{
			name: "SortAggregateName(desc)",
			q:    query.New[ID](query.SortBy(aggregate.SortName, aggregate.SortDesc)),
			want: []snapshot.Snapshot[ID]{
				snaps[8], snaps[7], snaps[6],
				snaps[5], snaps[4], snaps[3],
				snaps[2], snaps[1], snaps[0],
			},
		},
		{
			name: "SortAggregateID(asc)",
			q:    query.New[ID](query.SortBy(aggregate.SortID, aggregate.SortAsc)),
			want: snaps,
		},
		{
			name: "SortAggregateID(desc)",
			q:    query.New[ID](query.SortBy(aggregate.SortID, aggregate.SortDesc)),
			want: []snapshot.Snapshot[ID]{
				snaps[8], snaps[7], snaps[6],
				snaps[5], snaps[4], snaps[3],
				snaps[2], snaps[1], snaps[0],
			},
		},
		{
			name: "SortAggregateVersion(asc)",
			q:    query.New[ID](query.SortBy(aggregate.SortVersion, aggregate.SortAsc)),
			want: snaps,
		},
		{
			name: "SortAggregateVersion(desc)",
			q:    query.New[ID](query.SortBy(aggregate.SortVersion, aggregate.SortDesc)),
			want: []snapshot.Snapshot[ID]{
				snaps[8], snaps[7], snaps[6],
				snaps[5], snaps[4], snaps[3],
				snaps[2], snaps[1], snaps[0],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStore()
			for _, snap := range snaps {
				if err := store.Save(context.Background(), snap); err != nil {
					t.Fatalf("Save shouldn't fail; failed with %q", err)
				}
			}

			result, err := runQuery[ID](store, tt.q)
			if err != nil {
				t.Fatalf("query failed with %q", err)
			}

			assertEqual(t, tt.want, result)
		})
	}
}

func testDelete[ID goes.ID](t *testing.T, newStore StoreFactory[ID], newID func() ID) {
	s := newStore()
	a := &snapshotter[ID]{Base: aggregate.New("foo", newID())}
	snap, _ := snapshot.New[ID](a)

	if err := s.Save(context.Background(), snap); err != nil {
		t.Fatalf("Save shouldn't fail; failed with %q", err)
	}

	if err := s.Delete(context.Background(), snap); err != nil {
		t.Fatalf("Delete shouldn't fail; failed with %q", err)
	}

	snap, err := s.Latest(context.Background(), a.AggregateName(), a.AggregateID())
	if err == nil {
		t.Errorf("Latest should fail with an error; got %q", err)
	}
	if snap != nil {
		t.Errorf("Latest shouldn't return a Snapshot; got %v", snap)
	}
}

func runQuery[ID goes.ID](s snapshot.Store[ID], q snapshot.Query[ID]) ([]snapshot.Snapshot[ID], error) {
	str, errs, err := s.Query(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("expected store.Query to succeed; got %w", err)
	}
	return streams.Drain(context.Background(), str, errs)
}

func makeSnaps[ID goes.ID](as []aggregate.AggregateOf[ID]) []snapshot.Snapshot[ID] {
	snaps := make([]snapshot.Snapshot[ID], len(as))
	for i, a := range as {
		snap, err := snapshot.New(a)
		if err != nil {
			panic(err)
		}
		snaps[i] = snap
	}
	return snaps
}

func assertEqual[ID goes.ID](t *testing.T, want, got []snapshot.Snapshot[ID]) {
	if len(want) != len(got) {
		t.Fatalf("(len(want) == %d) != (len(got) == %d)", len(want), len(got))
	}

	for i, snap := range want {
		gs := got[i]
		if snap.AggregateName() != gs.AggregateName() {
			t.Errorf(
				"want[%d].AggregateName() == %q; got[%d].AggregateName() == %q",
				i,
				snap.AggregateName(),
				i,
				gs.AggregateName(),
			)
		}
		if snap.AggregateID() != gs.AggregateID() {
			t.Errorf(
				"want[%d].AggregateID() == %s; got[%d].AggregateID() == %s",
				i,
				snap.AggregateID(),
				i,
				gs.AggregateID(),
			)
		}
		if snap.AggregateVersion() != gs.AggregateVersion() {
			t.Errorf(
				"want[%d].AggregateVersion() == %d; got[%d].AggregateVersion() == %d",
				i,
				snap.AggregateVersion(),
				i,
				gs.AggregateVersion(),
			)
		}
	}
}

func assertSame[ID goes.ID](t *testing.T, want, got []snapshot.Snapshot[ID]) {
	sort.Slice(want, func(a, b int) bool {
		return want[a].AggregateID().String() <= want[b].AggregateID().String()
	})
	sort.Slice(got, func(a, b int) bool {
		return got[a].AggregateID().String() <= got[b].AggregateID().String()
	})
	assertEqual(t, want, got)
}

type snapshotter[ID goes.ID] struct {
	*aggregate.Base[ID]
	state state
}

type state struct {
	Foo int
}

func (ss *snapshotter[ID]) MarshalSnapshot() ([]byte, error) {
	return json.Marshal(ss.state)
}

func (ss *snapshotter[ID]) UnmarshalSnapshot(b []byte) error {
	return json.Unmarshal(b, &ss.state)
}
