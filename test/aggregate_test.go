package test_test

import (
	"errors"
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

func TestChange_notChanged(t *testing.T) {
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

func TestWithEventData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	foo := aggregate.New("foo", uuid.New())
	foo.TrackChange(event.New("evt", mockEventData{}))

	tt := mock_test.NewMockTestingT(ctrl)
	tt.EXPECT().Fatal(gomock.Any())

	test.Change(tt, foo, "evt", test.WithEventData(mockEventData{A: "foo"}))

	foo.TrackChange(event.New("evt", mockEventData{A: "foo"}))

	tt = mock_test.NewMockTestingT(ctrl)

	test.Change(tt, foo, "evt", test.WithEventData(mockEventData{A: "foo"}))
}

func TestAtLeast(t *testing.T) {
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

func TestAtMost(t *testing.T) {
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

func TestExactly(t *testing.T) {
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
