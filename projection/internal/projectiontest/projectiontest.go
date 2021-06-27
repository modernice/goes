package projectiontest

import (
	"reflect"
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection"
)

type MockProjection struct {
	AppliedEvents []event.Event
}

func NewMockProjection() *MockProjection {
	return &MockProjection{}
}

func (proj *MockProjection) ApplyEvent(evt event.Event) {
	proj.AppliedEvents = append(proj.AppliedEvents, evt)
}

func (proj *MockProjection) HasApplied(events ...event.Event) bool {
	for _, evt := range events {
		var applied bool
		for _, pevt := range proj.AppliedEvents {
			if reflect.DeepEqual(evt, pevt) {
				applied = true
				break
			}
		}
		if !applied {
			return false
		}
	}
	return true
}

func (proj *MockProjection) ExpectApplied(t *testing.T, events ...event.Event) {
	if !proj.HasApplied(events...) {
		t.Fatalf("mockProjection should have applied %v; has applied %v", events, proj.AppliedEvents)
	}
}

type MockProgressor struct {
	*MockProjection
	*projection.Progressor
}

func NewMockProgressor() *MockProgressor {
	return &MockProgressor{
		MockProjection: NewMockProjection(),
		Progressor:     &projection.Progressor{},
	}
}

type MockGuardedProjection struct {
	*MockProjection
	projection.Guard
}

func NewMockGuardedProjection(guard projection.Guard) *MockGuardedProjection {
	return &MockGuardedProjection{
		MockProjection: NewMockProjection(),
		Guard:          guard,
	}
}

type MockResetProjection struct {
	*MockProgressor

	Foo int
}

func NewMockResetProjection(val int) *MockResetProjection {
	return &MockResetProjection{
		MockProgressor: NewMockProgressor(),
		Foo:            val,
	}
}

func (proj *MockResetProjection) Reset() {
	proj.AppliedEvents = nil
	proj.Foo = 0
}
