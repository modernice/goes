package xaggregate_test

import (
	"testing"

	"github.com/modernice/goes/internal/xaggregate"
)

func TestMap(t *testing.T) {
	as, _ := xaggregate.Make(10)
	am := xaggregate.Map(as)

	if len(am) != len(as) {
		t.Errorf("aggregate map should contain %d elements; got %d", len(as), len(am))
	}

	for _, a := range as {
		a2, ok := am[a.AggregateID()]
		if !ok {
			t.Errorf("aggregate map should contain aggregate for id=%s", a.AggregateID())
		}
		if a != a2 {
			t.Errorf(
				"aggregate map contains the wrong aggregate for id=%s"+
					"\n\nwant: %#v\n\ngot: %#v\n\n",
				a.AggregateID(),
				a,
				a2,
			)
		}
	}
}
