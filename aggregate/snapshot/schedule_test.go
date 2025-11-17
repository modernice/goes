package snapshot_test

import (
	"fmt"
	"testing"

	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/event/test"
	"github.com/modernice/goes/internal"
	"github.com/modernice/goes/internal/xevent"
)

func TestEvery(t *testing.T) {
	tests := []struct {
		every      int
		oldVersion int
		newVersion int
		want       bool
	}{
		{
			every:      3,
			oldVersion: 0,
			newVersion: 3,
			want:       true,
		},
		{
			every:      3,
			oldVersion: 0,
			newVersion: 4,
			want:       true,
		},
		{
			every:      3,
			oldVersion: 0,
			newVersion: 2,
			want:       false,
		},
		{
			every:      3,
			oldVersion: 2,
			newVersion: 3,
			want:       true,
		},
		{
			every:      3,
			oldVersion: 2,
			newVersion: 4,
			want:       true,
		},
		{
			every:      3,
			oldVersion: 3,
			newVersion: 5,
			want:       false,
		},
		{
			every:      3,
			oldVersion: 3,
			newVersion: 6,
			want:       true,
		},
		{
			every:      3,
			oldVersion: 4,
			newVersion: 5,
			want:       false,
		},
		{
			every:      3,
			oldVersion: 4,
			newVersion: 6,
			want:       true,
		},
		{
			every:      3,
			oldVersion: 8,
			newVersion: 11,
			want:       true,
		},
		{
			every:      3,
			oldVersion: 6,
			newVersion: 9,
			want:       true,
		},
		{
			every:      3,
			oldVersion: 6,
			newVersion: 8,
			want:       false,
		},
		{
			every:      3,
			oldVersion: 5,
			newVersion: 8,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf(
			"Every(%d) old=%d new=%d",
			tt.every,
			tt.oldVersion,
			tt.newVersion,
		), func(t *testing.T) {
			s := snapshot.Every(tt.every)
			a := aggregate.New("foo", internal.NewUUID(), aggregate.Version(tt.oldVersion))
			events := xevent.Make("foo", test.FooEventData{}, tt.newVersion-tt.oldVersion, xevent.ForAggregate(a))
			for _, evt := range events {
				a.RecordChange(evt)
			}
			got := s.Test(a)
			if got != tt.want {
				t.Errorf("Test should return %v; got %v", tt.want, got)
			}
		})
	}
}
