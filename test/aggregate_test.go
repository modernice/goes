package test_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/test"
	"github.com/modernice/goes/test/mock_test"
)

type mockEventData struct {
	A string
}

func TestNewAggregate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	name := "foo"
	id2 := uuid.New()

	tests := []struct {
		new       func(uuid.UUID) aggregate.Aggregate
		wantError string
	}{
		{
			new: func(id uuid.UUID) aggregate.Aggregate {
				return aggregate.New(name, id)
			},
		},
		{
			new: func(id uuid.UUID) aggregate.Aggregate {
				return aggregate.New("bar", id)
			},
			wantError: fmt.Sprintf("AggregateName() should return %q; got %q", name, "bar"),
		},
		{
			new: func(id uuid.UUID) aggregate.Aggregate {
				return aggregate.New(name, id2)
			},
			wantError: fmt.Sprintf("AggregateID() should return %q; got %q", test.ExampleID, id2),
		},
	}

	for _, tt := range tests {
		mockT := mock_test.NewMockTestingT(ctrl)

		if tt.wantError != "" {
			mockT.EXPECT().Fatal(tt.wantError)
		}

		test.NewAggregate(mockT, tt.new, name)
	}
}

func TestChange_expectedChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())

	var err error
	var changeError *test.ExpectedChangeError

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any()).Do(func(args ...interface{}) {
		err, _ = args[0].(error)
	})

	test.Change(tt, foo, "evt")

	if !errors.As(err, &changeError) {
		t.Fatalf("Change() should fail with %T; got %#v", changeError, err)
	}

	if changeError.EventName != "evt" {
		t.Fatalf("ExpectedChangeError.EventName should be %q; is %q", "evt", changeError.EventName)
	}
}

func TestChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}))

	tt := mock_test.NewMockTestingT(ctrl)

	test.Change(tt, foo, "evt")
}

func TestChange_WithEventData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}))

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change(tt, foo, "evt", test.EventData(mockEventData{A: "foo"}))

	foo.TrackChange(event.New("evt", mockEventData{A: "foo"}))

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change(tt, foo, "evt", test.EventData(mockEventData{A: "foo"}))
}

func TestChange_AtLeast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())
	foo.TrackChange(
		event.New("evt", mockEventData{}),
		event.New("evt", mockEventData{}),
	)

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change(tt, foo, "evt", test.AtLeast(3))

	foo.TrackChange(event.New("evt", mockEventData{}))

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change(tt, foo, "evt", test.AtLeast(3))

	foo.TrackChange(event.New("evt", mockEventData{}))

	test.Change(tt, foo, "evt", test.AtLeast(3))
}

func TestChange_AtMost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())
	foo.TrackChange(
		event.New("evt", mockEventData{}),
		event.New("evt", mockEventData{}),
	)

	tt := mock_test.NewMockTestingT(ctrl)

	test.Change(tt, foo, "evt", test.AtMost(2))

	foo.TrackChange(event.New("evt", mockEventData{}))

	tt = mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change(tt, foo, "evt", test.AtMost(2))

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change(tt, foo, "evt", test.AtMost(3))

	foo.TrackChange(event.New("evt", mockEventData{}))

	test.Change(tt, foo, "evt", test.AtMost(4))

	foo.TrackChange(event.New("evt", mockEventData{}))

	tt = mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change(tt, foo, "evt", test.AtMost(4))
}

func TestChange_Exactly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())
	foo.TrackChange(
		event.New("evt", mockEventData{}),
		event.New("evt", mockEventData{}),
	)

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change(tt, foo, "evt", test.Exactly(3))

	foo.TrackChange(event.New("evt", mockEventData{}))

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change(tt, foo, "evt", test.Exactly(3))

	foo.TrackChange(event.New("evt", mockEventData{}))

	tt = mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change(tt, foo, "evt", test.Exactly(3))
}

func TestNoChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())

	tt := mock_test.NewMockTestingT(ctrl)

	test.NoChange(tt, foo, "evt")
}

func TestNoChange_unexpectedChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}))

	var err error
	var noChangeError *test.UnexpectedChangeError

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any()).Do(func(args ...interface{}) {
		err, _ = args[0].(error)
	})

	test.NoChange(tt, foo, "evt")

	if !errors.As(err, &noChangeError) {
		t.Fatalf("NoChange() should fail with %T; got %#v", noChangeError, err)
	}

	if noChangeError.EventName != "evt" {
		t.Fatalf("UnexpectedChangeError.EventName should be %q; is %q", "evt", noChangeError.EventName)
	}
}

func TestNoChange_WithEventData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}))

	tt := mock_test.NewMockTestingT(ctrl)
	test.NoChange(tt, foo, "evt", test.EventData(mockEventData{A: "foo"}))

	foo.TrackChange(event.New("evt", mockEventData{A: "foo"}))

	tt = mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.NoChange(tt, foo, "evt", test.EventData(mockEventData{A: "foo"}))
}
