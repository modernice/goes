package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/slice"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
)

// PermissionProjector continuously projects the Permissions read-model for all actors.
type PermissionProjector struct {
	schedule    *schedule.Continuous
	permissions PermissionRepository
	roles       RoleRepository
}

var projectorEvents = [...]string{
	PermissionGranted,
	PermissionRevoked,
	RoleGiven,
	RoleRemoved,
}

// NewPermissionProjector returns a new permission projector.
func NewPermissionProjector(
	perms PermissionRepository,
	roles RoleRepository,
	bus event.Bus,
	store event.Store,
	opts ...schedule.ContinuousOption,
) *PermissionProjector {
	return &PermissionProjector{
		schedule:    schedule.Continuously(bus, store, projectorEvents[:], opts...),
		permissions: perms,
		roles:       roles,
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
			if err := ctx.Apply(ctx, perms); err != nil {
				return err
			}

			if err := perms.finalize(ctx, proj.roles); err != nil {
				return fmt.Errorf("finalize permissions: %w", err)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("apply permissions: %w [actor=%v]", err, actorID)
		}
	}

	return nil
}

func (proj *PermissionProjector) extractActorsFromJob(ctx projection.Job) ([]uuid.UUID, error) {
	events, errs, err := ctx.Events(ctx)
	if err != nil {
		return nil, fmt.Errorf("extract events from job: %w", err)
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

			// Slowest path. We need to fetch each role and extract its members.
			case PermissionGranted, PermissionRevoked:
				actors, err := proj.getActorsOfRole(ctx, pick.AggregateID(evt))
				if err != nil {
					return fmt.Errorf("get actors of role: %w [roleId=%v]", err, pick.AggregateID(evt))
				}
				out = append(out, actors...)
			}
		}
		return nil
	}, events, errs); err != nil {
		return out, err
	}

	return slice.Unique(out), nil
}

func (proj *PermissionProjector) getActorsOfRole(ctx context.Context, roleID uuid.UUID) ([]uuid.UUID, error) {
	role, err := proj.roles.Fetch(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("fetch role: %w [id=%v]", err, roleID)
	}
	return role.members, nil
}
