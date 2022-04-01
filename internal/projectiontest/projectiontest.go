package projectiontest

import (
	"reflect"
	"testing"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection"
)

// MockProjection is a projection mock that's used for testing.
type MockProjection struct {
	AppliedEvents []event.Event
}

// NewMockProjection returns a MockProjection.
func NewMockProjection() *MockProjection {
	return &MockProjection{}
}

// ApplyEvent adds evt as an applied event to the MockProjection.
func (proj *MockProjection) ApplyEvent(evt event.Event) {
	proj.AppliedEvents = append(proj.AppliedEvents, evt)
}

// HasApplied determines whether the passed events have been applied onto the
// MockProjection.
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

// ExpectApplied determines whether events were applied onto the MockProjection
// and if not, makes the test fail.
func (proj *MockProjection) ExpectApplied(t *testing.T, events ...event.Event) {
	if !proj.HasApplied(events...) {
		t.Fatalf("mockProjection should have applied %v; has applied %v", events, proj.AppliedEvents)
	}
}

// MockProgressor is a projection mock that embeds a *projection.Progressor.
type MockProgressor struct {
	*MockProjection
	*projection.Progressor
}

// NewMockProgressor returns a MockProgressor.
func NewMockProgressor() *MockProgressor {
	return &MockProgressor{
		MockProjection: NewMockProjection(),
		Progressor:     &projection.Progressor{},
	}
}

// MockGuardedProjection is a projection mock that embeds a projection.Guard.
type MockGuardedProjection struct {
	*MockProjection
	projection.QueryGuard
}

// NewMockGuardedProjection returns a MockGuardedProjection.
func NewMockGuardedProjection(guard projection.QueryGuard) *MockGuardedProjection {
	return &MockGuardedProjection{
		MockProjection: NewMockProjection(),
		QueryGuard:     guard,
	}
}

// MockResetProjection is a projection mock that embeds a *projection.Progressor
// and has a Reset method.
type MockResetProjection struct {
	*MockProgressor

	Foo int
}

// NewMockResetProjection returns a MockResetProjection. The Foo field of the
// projection is set to val. When the Reset method of the projection is called,
// Foo is set to 0 to mock a reset of the projection.
func NewMockResetProjection(val int) *MockResetProjection {
	return &MockResetProjection{
		MockProgressor: NewMockProgressor(),
		Foo:            val,
	}
}

// Reset resets the projection by clearing its applied events and setting the
// Foo field to 0.
func (proj *MockResetProjection) Reset() {
	proj.AppliedEvents = nil
	proj.Foo = 0
}
