package event_test

import (
	"fmt"
	"testing"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal"
)

func TestAggregateRef_Parse(t *testing.T) {
	validID := internal.NewUUID()

	tests := []struct {
		name      string
		value     string
		wantRef   aggregate.Ref
		wantError bool
	}{
		{
			name:  "valid",
			value: fmt.Sprintf("goes.test.foo(%s)", validID),
			wantRef: event.AggregateRef{
				Name: "goes.test.foo",
				ID:   validID,
			},
		},
		{
			name:      "empty string",
			value:     "",
			wantError: true,
		},
		{
			name:      "invalid id",
			value:     fmt.Sprintf("goes.test.foo(%s)", validID.String()[:8]),
			wantError: true,
		},
		{
			name:      "missing name",
			value:     fmt.Sprintf("(%s)", validID),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ref event.AggregateRef
			err := ref.Parse(tt.value)

			if tt.wantError && (err == nil || err.Error() != fmt.Sprintf("invalid ref string: %q", tt.value)) {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantRef != ref {
				t.Fatalf("unexpected ref: got %v, want %v", ref, tt.wantRef)
			}
		})
	}
}
