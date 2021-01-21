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
)

var _ event.Query = Query{}

func TestQuery(t *testing.T) {
	q := New()

	if reflect.TypeOf(q.Names()) != reflect.TypeOf([]string{}) {
		t.Errorf("expected type of q.Names() to be %T; got %T", []string{}, q.Names())
	}

	if reflect.TypeOf(q.IDs()) != reflect.TypeOf([]uuid.UUID{}) {
		t.Errorf("expected type of q.IDs() to be %T; got %T", []uuid.UUID{}, q.IDs())
	}

	if reflect.TypeOf(q.Times()) != reflect.TypeOf(time.Filter()) {
		t.Errorf("expected type of q.Times() to be %T; got %T", time.Filter(), q.Times())
	}

	if reflect.TypeOf(q.AggregateNames()) != reflect.TypeOf([]string{}) {
		t.Errorf("expected type of q.AggregateNames() to be %T; got %T", []string{}, q.AggregateNames())
	}

	if reflect.TypeOf(q.AggregateIDs()) != reflect.TypeOf([]uuid.UUID{}) {
		t.Errorf("expected type of q.AggregateIDs() to be %T; got %T", []uuid.UUID{}, q.AggregateIDs())
	}

	if reflect.TypeOf(q.AggregateVersions()) != reflect.TypeOf(version.Filter()) {
		t.Errorf("expected type of q.AggregateVersions() to be %T; got %T", version.Filter(), q.AggregateVersions())
	}
}

func TestNew(t *testing.T) {
	ids := make([]uuid.UUID, 4)
	times := make([]stdtime.Time, 4)
	now := stdtime.Now()
	for i := range ids {
		ids[i] = uuid.New()
		times[i] = now.Add(stdtime.Duration(i) * stdtime.Minute)
	}

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
			name: "SortBy",
			opts: []Option{
				SortBy(event.SortTime, event.SortAsc),
			},
			want: Query{
				times:             time.Filter(),
				aggregateVersions: version.Filter(),
				sorting: event.SortConfig{
					Sort: event.SortTime,
					Dir:  event.SortAsc,
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
	now := stdtime.Now()
	for i := range ids {
		ids[i] = uuid.New()
		times[i] = now.Add(stdtime.Duration(i) * stdtime.Minute)
	}

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
				event.New("foo", test.FooEventData{}, event.Aggregate("foo", uuid.New(), 0)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate("bar", uuid.New(), 0)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate("baz", uuid.New(), 0)): false,
			},
		},
		{
			name:  "AggregateID",
			query: New(AggregateID(ids[:2]...)),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate("foo", ids[0], 0)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate("bar", ids[1], 0)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate("baz", ids[2], 0)): false,
			},
		},
		{
			name:  "AggregateVersion (exact)",
			query: New(AggregateVersion(version.Exact(1, 2))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate("foo", uuid.New(), 1)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate("bar", uuid.New(), 2)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate("baz", uuid.New(), 3)): false,
			},
		},
		{
			name:  "AggregateVersion (range)",
			query: New(AggregateVersion(version.InRange(version.Range{1, 2}))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate("foo", uuid.New(), 1)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate("bar", uuid.New(), 2)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate("baz", uuid.New(), 3)): false,
			},
		},
		{
			name:  "AggregateVersion (min/max)",
			query: New(AggregateVersion(version.Min(1), version.Max(2))),
			tests: map[event.Event]bool{
				event.New("foo", test.FooEventData{}, event.Aggregate("foo", uuid.New(), 1)): true,
				event.New("bar", test.BarEventData{}, event.Aggregate("bar", uuid.New(), 2)): true,
				event.New("baz", test.BazEventData{}, event.Aggregate("baz", uuid.New(), 3)): false,
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
