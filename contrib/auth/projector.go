package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/slice"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

// PermissionProjector continuously projects the Permissions read-model for all actors.
type PermissionProjector struct {
	schedule    *schedule.Continuous
	permissions PermissionRepository
}

var projectorEvents = [...]string{
	PermissionGranted,
	PermissionRevoked,
	RoleGiven,
	RoleRemoved,
}

// NewPermissionProjector returns a new permission projector.
func NewPermissionProjector(perms PermissionRepository, bus event.Bus, store event.Store, opts ...schedule.ContinuousOption) *PermissionProjector {
	return &PermissionProjector{
		schedule:    schedule.Continuously(bus, store, projectorEvents[:], opts...),
		permissions: perms,
	}
}

// Run projects permissions until ctx is canceled.
func (proj *PermissionProjector) Run(ctx context.Context) (<-chan error, error) {
	errs, err := proj.schedule.Subscribe(ctx, proj.applyJob)
	if err != nil {
		return nil, fmt.Errorf("subscribe to projection schedule: %w", err)
	}

	go proj.schedule.Trigger(ctx)

	return errs, nil
}

func (proj *PermissionProjector) applyJob(ctx projection.Job) error {
	actors, err := proj.extractActorsFromJob(ctx)
	if err != nil {
		return fmt.Errorf("extract actors from job: %w", err)
	}

	for _, actorID := range actors {
		if err := proj.permissions.Use(ctx, actorID, func(perms *Permissions) error {
			return ctx.Apply(ctx, perms)
		}); err != nil {
			return fmt.Errorf("apply permissions: %w [actor=%v]", err, actorID)
		}
	}

	return nil
}

func (proj *PermissionProjector) extractActorsFromJob(ctx projection.Job) ([]uuid.UUID, error) {
	events, errs, err := ctx.EventsOf(ctx, ActorAggregate, RoleAggregate)
	if err != nil {
		return nil, fmt.Errorf("extract events of %v aggregates: %w", []string{ActorAggregate, RoleAggregate}, err)
	}

	var out []uuid.UUID
	if err := streams.Walk(ctx, func(evt event.Event) error {
		id, name, _ := evt.Aggregate()
		switch name {
		case ActorAggregate:
			out = append(out, id)
		case RoleAggregate:
			switch evt.Name() {
			case RoleGiven, RoleRemoved:
				out = append(out, evt.Data().([]uuid.UUID)...)
			}
		}
		return nil
	}, events, errs); err != nil {
		return out, err
	}

	return slice.Unique(out), nil
}
