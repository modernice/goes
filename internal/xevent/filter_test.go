package xevent_test

import (
	"testing"

	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal/xaggregate"
	"github.com/modernice/goes/internal/xevent"
)

func TestFilterAggregate(t *testing.T) {
	as, _ := xaggregate.Make(1)
	naevents := xevent.Make("foo", test.FooEventData{}, 10)
	aevents := xevent.Make("foo", test.FooEventData{}, 10, xevent.ForAggregate(as[0]))
	events := append(naevents, aevents...)

	filtered := xevent.FilterAggregate(events, as[0])

	test.AssertEqualEventsUnsorted(t, filtered, aevents)
}
