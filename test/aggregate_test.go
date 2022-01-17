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
		new       func(uuid.UUID) aggregate.Aggregate[any]
		wantError string
	}{
		{
			new: func(id uuid.UUID) aggregate.Aggregate[any] {
				return aggregate.New[any](name, id)
			},
		},
		{
			new: func(id uuid.UUID) aggregate.Aggregate[any] {
				return aggregate.New[any]("bar", id)
			},
			wantError: fmt.Sprintf("AggregateName() should return %q; got %q", name, "bar"),
		},
		{
			new: func(id uuid.UUID) aggregate.Aggregate[any] {
				return aggregate.New[any](name, id2)
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

	foo := aggregate.New[any]("foo", uuid.New())

	var err error
	var changeError *test.ExpectedChangeError[any]

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any()).Do(func(args ...any) {
		err, _ = args[0].(error)
	})

	test.Change[any](tt, foo, "evt")

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

	foo := aggregate.New[any]("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	tt := mock_test.NewMockTestingT(ctrl)

	test.Change[any](tt, foo, "evt")
}

func TestChange_WithEventData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New[any]("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change[any](tt, foo, "evt", test.EventData[any](mockEventData{A: "foo"}))

	foo.TrackChange(event.New("evt", mockEventData{A: "foo"}).Any())

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change[any](tt, foo, "evt", test.EventData[any](mockEventData{A: "foo"}))
}

func TestChange_AtLeast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New[any]("foo", uuid.New())
	foo.TrackChange(
		event.New("evt", mockEventData{}).Any(),
		event.New("evt", mockEventData{}).Any(),
	)

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change[any](tt, foo, "evt", test.AtLeast[any](3))

	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change[any](tt, foo, "evt", test.AtLeast[any](3))

	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	test.Change[any](tt, foo, "evt", test.AtLeast[any](3))
}

func TestChange_AtMost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New[any]("foo", uuid.New())
	foo.TrackChange(
		event.New("evt", mockEventData{}).Any(),
		event.New("evt", mockEventData{}).Any(),
	)

	tt := mock_test.NewMockTestingT(ctrl)

	test.Change[any](tt, foo, "evt", test.AtMost[any](2))

	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	tt = mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change[any](tt, foo, "evt", test.AtMost[any](2))

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change[any](tt, foo, "evt", test.AtMost[any](3))

	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	test.Change[any](tt, foo, "evt", test.AtMost[any](4))

	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	tt = mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change[any](tt, foo, "evt", test.AtMost[any](4))
}

func TestChange_Exactly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New[any]("foo", uuid.New())
	foo.TrackChange(
		event.New("evt", mockEventData{}).Any(),
		event.New("evt", mockEventData{}).Any(),
	)

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change[any](tt, foo, "evt", test.Exactly[any](3))

	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change[any](tt, foo, "evt", test.Exactly[any](3))

	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	tt = mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change[any](tt, foo, "evt", test.Exactly[any](3))
}

func TestNoChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New[any]("foo", uuid.New())

	tt := mock_test.NewMockTestingT(ctrl)

	test.NoChange[any](tt, foo, "evt")
}

func TestNoChange_unexpectedChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New[any]("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	var err error
	var noChangeError *test.UnexpectedChangeError

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any()).Do(func(args ...any) {
		err, _ = args[0].(error)
	})

	test.NoChange[any](tt, foo, "evt")

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

	foo := aggregate.New[any]("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}).Any())

	tt := mock_test.NewMockTestingT(ctrl)
	test.NoChange[any](tt, foo, "evt", test.EventData[any](mockEventData{A: "foo"}))

	foo.TrackChange(event.New("evt", mockEventData{A: "foo"}).Any())

	tt = mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.NoChange[any](tt, foo, "evt", test.EventData[any](mockEventData{A: "foo"}))
}
