package tagging_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/aggregate/tagging"
	"github.com/modernice/goes/aggregate/tagging/memtags"
	"github.com/modernice/goes/aggregate/tagging/mock_tagging"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore/memstore"
	mock_event "github.com/modernice/goes/event/mocks"
	equery "github.com/modernice/goes/event/query"
)

func TestPlugin_save(t *testing.T) {
	estore := memstore.New()
	tags := memtags.NewStore()

	repo := repository.New(estore, tagging.Plugin(tags))

	foo := newTagger()
	tagging.Tag(foo, "foo", "bar")

	if err := repo.Save(context.Background(), foo); err != nil {
		t.Fatalf("Save failed with %q", err)
	}

	storeTags, err := tags.Tags(context.Background(), foo.Name, foo.ID)
	if err != nil {
		t.Fatalf("tag store: %v", err)
	}

	want := []string{"foo", "bar"}
	assertTags(t, want, storeTags)
}

func TestPlugin_rollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	estore := mock_event.NewMockStore(ctrl)
	realEventStore := memstore.New()
	tags := memtags.NewStore()

	plugin := tagging.Plugin(tags)
	repo := repository.New(estore, plugin)
	realRepo := repository.New(realEventStore, plugin)

	foo := newTagger()
	tagging.Tag(foo, "foo", "bar")

	if err := realRepo.Save(context.Background(), foo); err != nil {
		t.Fatalf("Save failed with %q", err)
	}

	storeTags, err := tags.Tags(context.Background(), foo.AggregateName(), foo.AggregateID())
	if err != nil {
		t.Fatalf("Tags failed with %q", err)
	}

	assertTags(t, []string{"foo", "bar"}, storeTags)

	tagging.Untag(foo, "foo")
	tagging.Tag(foo, "baz")

	mockError := errors.New("mock error")
	estore.EXPECT().
		Insert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, ...event.Event) error {
			return mockError
		})

	if err := repo.Save(context.Background(), foo); !errors.Is(err, mockError) {
		t.Fatalf("Save failed with %q", err)
	}

	storeTags, err = tags.Tags(context.Background(), foo.AggregateName(), foo.AggregateID())
	if err != nil {
		t.Fatalf("Tags failed with %q", err)
	}

	assertTags(t, []string{"bar", "baz"}, storeTags)
}

func TestPlugin_delete(t *testing.T) {
	estore := memstore.New()
	tagStore := memtags.NewStore()
	plugin := tagging.Plugin(tagStore)
	r := repository.New(estore, plugin)

	foo := newTagger()
	tagging.Tag(foo, "foo", "bar")

	if err := r.Save(context.Background(), foo); err != nil {
		t.Fatalf("Save failed with %q", err)
	}

	if err := r.Delete(context.Background(), foo); err != nil {
		t.Fatalf("Delete failed with %q", err)
	}

	tags, err := tagStore.Tags(context.Background(), foo.AggregateName(), foo.AggregateID())
	if err != nil {
		t.Fatalf("Tags failed with %q", err)
	}

	if len(tags) != 0 {
		t.Fatalf("Tag should return 0 tags; got %d (%v)", len(tags), tags)
	}
}

func TestPlugin_delete_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	estore := memstore.New()
	tagStore := mock_tagging.NewMockStore(ctrl)
	plugin := tagging.Plugin(tagStore)
	repo := repository.New(estore, plugin)

	foo := newTagger()

	mockError := errors.New("mock error")
	tagStore.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockError)

	if err := repo.Delete(context.Background(), foo); !errors.Is(err, mockError) {
		t.Fatalf("Delete should fail with %q; got %q", mockError, err)
	}
}

func TestPlugin_query(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	estore := mock_event.NewMockStore(ctrl)
	tagStore := mock_tagging.NewMockStore(ctrl)
	plugin := tagging.Plugin(tagStore)
	repo := repository.New(estore, plugin)

	aggregates := []tagging.Aggregate{
		{Name: "foo", ID: uuid.New()},
		{Name: "foo", ID: uuid.New()},
		{Name: "bar", ID: uuid.New()},
		{Name: "bar", ID: uuid.New()},
	}

	tagStore.EXPECT().
		TaggedWith(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, ...string) ([]tagging.Aggregate, error) {
			return aggregates, nil
		})

	var opts []equery.Option
	for _, a := range aggregates {
		opts = append(opts, equery.Aggregate(a.Name, a.ID))
	}

	wantQuery := equery.Merge(equery.New(opts...), equery.New(equery.SortByAggregate()))
	estore.EXPECT().Query(gomock.Any(), wantQuery).DoAndReturn(func(context.Context, event.Query) (<-chan event.Event, <-chan error, error) {
		events := make(chan event.Event)
		errs := make(chan error)
		close(events)
		close(errs)
		return events, errs, nil
	})

	repo.Query(context.Background(), query.New(query.Tag("foo", "bar")))
}

func assertTags(t *testing.T, want, got []string) {
	if len(want) != len(got) {
		t.Fatalf("expected %d tags; got %d", len(want), len(got))
	}

	for _, name := range want {
		var found bool
		for _, tag := range got {
			if tag == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q tag", name)
		}
	}
}
