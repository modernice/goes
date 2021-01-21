package repository_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/aggregate/test"
	"github.com/modernice/goes/event"
	eventtest "github.com/modernice/goes/event/test"
)

func TestSaveError_Error(t *testing.T) {
	a := test.NewFoo(uuid.New())
	mockError := errors.New("mock error")

	tests := map[*repository.SaveError]string{
		{
			Aggregate: a,
			Err:       mockError,
		}: "save: mock error; rollbacks=0 failed=0",

		{
			Aggregate: a,
			Err:       mockError,
			Rollbacks: repository.SaveRollbacks{
				{Event: event.New("foo", eventtest.FooEventData{})},
				{Event: event.New("foo", eventtest.FooEventData{})},
				{Event: event.New("foo", eventtest.FooEventData{})},
			},
		}: "save: mock error; rollbacks=3 failed=0",

		{
			Aggregate: a,
			Err:       mockError,
			Rollbacks: repository.SaveRollbacks{
				{Event: event.New("foo", eventtest.FooEventData{}), Err: mockError},
				{Event: event.New("foo", eventtest.FooEventData{})},
				{Event: event.New("foo", eventtest.FooEventData{}), Err: mockError},
			},
		}: "save: mock error; rollbacks=3 failed=2",

		{
			Aggregate: a,
			Err:       mockError,
			Rollbacks: repository.SaveRollbacks{
				{Event: event.New("foo", eventtest.FooEventData{}), Err: mockError},
				{Event: event.New("foo", eventtest.FooEventData{}), Err: mockError},
				{Event: event.New("foo", eventtest.FooEventData{}), Err: mockError},
			},
		}: "save: mock error; rollbacks=3 failed=3",
	}

	for give, want := range tests {
		if give.Error() != want {
			t.Errorf("expected give.Error to return %q; got %q", want, give.Error())
		}
	}
}
