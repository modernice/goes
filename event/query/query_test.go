package query

import (
	"reflect"
	"testing"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xtime"
)

var _ event.Query = Query{}

func TestNew(t *testing.T) {
	ids := make([]uuid.UUID, 4)
	times := make([]stdtime.Time, 4)
	now := xtime.Now()
	for i := range ids {
		ids[i] = uuid.New()
		times[i] = now.Add(stdtime.Duration(i) * stdtime.Minute)
	}

	aggregateID := uuid.New()

	tests := []struct {
		name string
		opts []Option
		want Query
	}{
		{
			name: "Name",
			opts: []Option{
				Name("foo", "bar"),
				Name("baz", "foobar"),
			},
			want: Query{
				times:             time.Filter(),
				aggregateVersions: version.Filter(),
				names:             []string{"foo", "bar", "baz", "foobar"},
			},
		},
		{
			name: "ID",
			opts: []Option{
				ID(ids[:2]...),
				ID(ids[2:4]...),
			},
			want: Query{
				times:             time.Filter(),
				aggregateVersions: version.Filter(),
				ids:               ids,
			},
		},
		{
			name: "Time",
			opts: []Option{
				Time(time.Exact(times[2:]...)),
				Time(time.InRange(time.Range{times[2], times[3]})),
			},
			want: Query{
				aggregateVersions: version.Filter(),
				times: time.Filter(
					time.Exact(times[2:]...),
					time.InRange(time.Range{times[2], times[3]}),
				),
			},
		},
		{
			name: "AggregateName",
			opts: []Option{
				AggregateName("foo", "bar"),
				AggregateName("baz", "foobar"),
			},
			want: Query{
				times:             time.Filter(),
				aggregateVersions: version.Filter(),
				aggregateNames:    []string{"foo", "bar", "baz", "foobar"},
			},
		},
		{
			name: "AggregateID",
			opts: []Option{
				AggregateID(ids[:2]...),
				AggregateID(ids[2:]...),
			},
			want: Query{
				times:             time.Filter(),
				aggregateVersions: version.Filter(),
				aggregateIDs:      ids,
			},
		},
		{
			name: "AggregateVersion",
			opts: []Option{
				AggregateVersion(version.Exact(1, 2, 3)),
				AggregateVersion(version.InRange(version.Range{10, 20})),
			},
			want: Query{
				times: time.Filter(),
				aggregateVersions: version.Filter(
					version.Exact(1, 2, 3),
					version.InRange(version.Range{10, 20}),
				),
			},
		},
		{
			name: "Aggregate",
			opts: []Option{
				Aggregate("foo", aggregateID),
				Aggregate("bar", aggregateID),
			},
			want: Query{
				times:             time.Filter(),
				aggregateVersions: version.Filter(),
				aggregates: []event.AggregateTuple{
					{
						Name: "foo",
						ID:   aggregateID,
					},
					{
						Name: "bar",
						ID:   aggregateID,
					},
				},
			},
		},
		{
			name: "SortBy",
			opts: []Option{
				SortBy(event.SortTime, event.SortAsc),
			},
			want: Query{
				times:             time.Filter(),
				aggregateVersions: version.Filter(),
				sortings: []event.SortOptions{{
					Sort: event.SortTime,
					Dir:  event.SortAsc,
				}},
			},
		},
		{
			name: "SortByMulti",
			opts: []Option{
				SortByMulti(
					event.SortOptions{Sort: event.SortTime, Dir: event.SortAsc},
					event.SortOptions{Sort: event.SortAggregateVersion, Dir: event.SortDesc},
				),
			},
			want: Query{
				times:             time.Filter(),
				aggregateVersions: version.Filter(),
				sortings: []event.SortOptions{
					{
						Sort: event.SortTime,
						Dir:  event.SortAsc,
					},
					{
						Sort: event.SortAggregateVersion,
						Dir:  event.SortDesc,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := New(tt.opts...)
			if !reflect.DeepEqual(q, tt.want) {
				t.Errorf("returned query doesn't match expected\ngot: %#v\n\nexpected: %#v", q, tt.want)
			}
		})
	}
}

func TestTest(t *testing.T) {
	ids := make([]uuid.UUID, 4)
	times := make([]stdtime.Time, 4)
	now := xtime.Now()
	for i := range ids {
		ids[i] = uuid.New()
		times[i] = now.Add(stdtime.Duration(i) * stdtime.Minute)
	}

	aggregateID := uuid.New()

	tests := []struct {
		name  string
		query event.Query
		tests map[event.Event]bool
	}{
		{
			name:  "Name",
			query: New(Name("foo", "bar")),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}): true,
				event.New("bar", test.BarEventData{}): true,
				event.New("baz", test.BazEventData{}): false,
			},
		},
		{
			name:  "ID",
			query: New(ID(ids[:2]...)),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.ID(ids[0])): true,
				event.New("bar", test.BarEventData{}, event.ID(ids[1])): true,
				event.New("baz", test.BazEventData{}, event.ID(ids[2])): false,
			},
		},
		{
			name:  "Time (exact)",
			query: New(Time(time.Exact(times[:2]...))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Time(times[0])): true,
				event.New("bar", test.BarEventData{}, event.Time(times[1])): true,
				event.New("baz", test.BazEventData{}, event.Time(times[2])): false,
			},
		},
		{
			name:  "Time (range)",
			query: New(Time(time.InRange(time.Range{times[0], times[1]}))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Time(times[0])): true,
				event.New("bar", test.BarEventData{}, event.Time(times[1])): true,
				event.New("baz", test.BazEventData{}, event.Time(times[2])): false,
			},
		},
		{
			name:  "Time (min/max)",
			query: New(Time(time.Min(times[0]), time.Max(times[1]))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Time(times[0])): true,
				event.New("bar", test.BarEventData{}, event.Time(times[1])): true,
				event.New("baz", test.BazEventData{}, event.Time(times[2])): false,
			},
		},
		{
			name:  "AggregateName",
			query: New(AggregateName("foo", "bar")),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 0)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate(uuid.New(), "bar", 0)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate(uuid.New(), "baz", 0)): false,
			},
		},
		{
			name:  "AggregateID",
			query: New(AggregateID(ids[:2]...)),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate(ids[0], "foo", 0)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate(ids[1], "bar", 0)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate(ids[2], "baz", 0)): false,
			},
		},
		{
			name:  "AggregateVersion (exact)",
			query: New(AggregateVersion(version.Exact(1, 2))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 1)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate(uuid.New(), "bar", 2)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate(uuid.New(), "baz", 3)): false,
			},
		},
		{
			name:  "AggregateVersion (range)",
			query: New(AggregateVersion(version.InRange(version.Range{1, 2}))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 1)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate(uuid.New(), "bar", 2)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate(uuid.New(), "baz", 3)): false,
			},
		},
		{
			name:  "AggregateVersion (min/max)",
			query: New(AggregateVersion(version.Min(1), version.Max(2))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 1)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate(uuid.New(), "bar", 2)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate(uuid.New(), "baz", 3)): false,
			},
		},
		{
			name:  "Aggregate",
			query: New(Aggregate("foo", aggregateID)),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}):                                         false,
				event.New("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 0)):  false,
				event.New("foo", test.FooEventData{}, event.Aggregate(aggregateID, "foo", 0)): true,
				event.New("foo", test.FooEventData{}, event.Aggregate(aggregateID, "foo", 4)): true,
			},
		},
		{
			name:  "Aggregate (uuid.Nil)",
			query: New(Aggregate("foo", aggregateID), Aggregate("bar", uuid.Nil)),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}):                                         false,
				event.New("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "foo", 0)):  false,
				event.New("foo", test.FooEventData{}, event.Aggregate(aggregateID, "foo", 0)): true,
				event.New("foo", test.FooEventData{}, event.Aggregate(aggregateID, "foo", 4)): true,
				event.New("foo", test.FooEventData{}, event.Aggregate(uuid.New(), "bar", 0)):  true,
				event.New("foo", test.FooEventData{}, event.Aggregate(aggregateID, "bar", 0)): true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for evt, want := range tt.tests {
				if got := Test(tt.query, evt); got != want {
					t.Errorf("expected query.Test to return %t; got %t", want, got)
				}
			}
		})
	}
}

func TestMerge(t *testing.T) {
	now := xtime.Now()
	queries := []event.Query{
		Query{
			names:             []string{"foo"},
			ids:               []uuid.UUID{uuid.New()},
			aggregateNames:    []string{"foo"},
			aggregateIDs:      []uuid.UUID{uuid.New()},
			sortings:          []event.SortOptions{{Sort: event.SortAggregateName, Dir: event.SortDesc}},
			times:             time.Filter(time.After(now)),
			aggregateVersions: version.Filter(version.Exact(1)),
			aggregates:        []event.AggregateTuple{{Name: "foo", ID: uuid.New()}},
		},
		Query{
			names:             []string{"bar"},
			ids:               []uuid.UUID{uuid.New()},
			aggregateNames:    []string{"bar"},
			aggregateIDs:      []uuid.UUID{uuid.New()},
			sortings:          []event.SortOptions{{Sort: event.SortAggregateID, Dir: event.SortAsc}},
			times:             time.Filter(time.Before(now)),
			aggregateVersions: version.Filter(version.Exact(2, 3)),
			aggregates:        []event.AggregateTuple{{Name: "bar", ID: uuid.New()}},
		},
		Query{
			names:             []string{"baz"},
			ids:               []uuid.UUID{uuid.New()},
			aggregateNames:    []string{"baz"},
			aggregateIDs:      []uuid.UUID{uuid.New()},
			sortings:          []event.SortOptions{{Sort: event.SortAggregateVersion, Dir: event.SortAsc}},
			times:             time.Filter(time.Exact(now)),
			aggregateVersions: version.Filter(version.Exact(4, 5)),
			aggregates:        []event.AggregateTuple{{Name: "baz", ID: uuid.New()}},
		},
	}

	q := Merge(queries...)

	wantNames := []string{"foo", "bar", "baz"}
	if !reflect.DeepEqual(q.Names(), wantNames) {
		t.Fatalf("Names should return %v; got %v", wantNames, q.Names())
	}

	if !reflect.DeepEqual(q.AggregateNames(), wantNames) {
		t.Fatalf("AggregateNames should return %v; got %v", wantNames, q.AggregateNames())
	}

	wantIDs := append(queries[0].IDs(), queries[1].IDs()...)
	wantIDs = append(wantIDs, queries[2].IDs()...)

	if !reflect.DeepEqual(q.IDs(), wantIDs) {
		t.Fatalf("IDs should return %v; got %v", wantIDs, q.IDs())
	}

	wantIDs = append(queries[0].AggregateIDs(), queries[1].AggregateIDs()...)
	wantIDs = append(wantIDs, queries[2].AggregateIDs()...)

	if !reflect.DeepEqual(q.AggregateIDs(), wantIDs) {
		t.Fatalf("AggregateIDs should return %v; got %v", wantIDs, q.AggregateIDs())
	}

	wantSortings := []event.SortOptions{
		{Sort: event.SortAggregateName, Dir: event.SortDesc},
		{Sort: event.SortAggregateID, Dir: event.SortAsc},
		{Sort: event.SortAggregateVersion, Dir: event.SortAsc},
	}
	if !reflect.DeepEqual(q.Sortings(), wantSortings) {
		t.Fatalf("Sortings should return %v; got %v", wantSortings, q.Sortings())
	}

	wantTimes := time.Filter(time.After(now), time.Before(now), time.Exact(now))
	if !reflect.DeepEqual(q.Times(), wantTimes) {
		t.Fatalf("Times should return %v; got %v", wantTimes, q.Times())
	}

	wantVersions := version.Filter(version.Exact(1, 2, 3, 4, 5))
	if !reflect.DeepEqual(q.AggregateVersions(), wantVersions) {
		t.Fatalf("Versions should return %v; got %v", wantVersions, q.AggregateVersions())
	}

	wantAggregates := append(queries[0].Aggregates(), queries[1].Aggregates()...)
	wantAggregates = append(wantAggregates, queries[2].Aggregates()...)
	if !reflect.DeepEqual(q.Aggregates(), wantAggregates) {
		t.Fatalf("Aggregates should return %v; got %v", wantAggregates, q.Aggregates())
	}
}
