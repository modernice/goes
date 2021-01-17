package query

import (
	"reflect"
	"testing"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/time"
	"github.com/modernice/goes/event/version"
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
