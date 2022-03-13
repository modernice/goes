package authpb

import (
	"fmt"

	"github.com/modernice/goes/aggregate"
	commonpb "github.com/modernice/goes/api/proto/gen/common"
	"github.com/modernice/goes/contrib/auth"
)

// NewActions converts auth.Actions to *Actions.
func NewActions(actions auth.Actions) *Actions {
	m := make(map[string]*ActionGrants)
	for ref, actions := range actions {
		m[ref.String()] = newActionGrants(actions)
	}
	return &Actions{Actions: m}
}

// AsMap converts *Actions to auth.Actions, ignoring any errors.
func (a *Actions) AsMap() auth.Actions {
	out, _ := a.ToMap()
	return out
}

// ToMap converts *Actions to auth.Actions. ToMap returns an error if a string
// key of the map is not a valid aggregate reference.
func (a *Actions) ToMap() (auth.Actions, error) {
	out := make(auth.Actions)
	for refval, grants := range a.GetActions() {
		var ref aggregate.Ref
		if err := ref.Parse(refval); err != nil {
			return out, fmt.Errorf("parse %q ref: %w", refval, err)
		}
		out[ref] = grants.asMap()
	}
	return out, nil
}

// NewPermissions converts auth.Permissions to *Permissions.
func NewPermissions(perms auth.PermissionsDTO) *Permissions {
	return &Permissions{
		ActorId: commonpb.NewUUID(perms.ActorID),
		OfActor: NewActions(perms.OfActor),
		OfRoles: NewActions(perms.OfRoles),
	}
}

// AsDTO converts *Permissions to auth.PermissionsDTO.
func (perms *Permissions) AsDTO() auth.PermissionsDTO {
	return auth.PermissionsDTO{
		ActorID: perms.GetActorId().AsUUID(),
		OfActor: perms.GetOfActor().AsMap(),
		OfRoles: perms.GetOfRoles().AsMap(),
	}
}

func newActionGrants(actions map[string]int) *ActionGrants {
	m := make(map[string]uint64)
	for action, count := range actions {
		m[action] = uint64(count)
	}
	return &ActionGrants{Actions: m}
}

func (pa *ActionGrants) asMap() map[string]int {
	out := make(map[string]int)
	for action, count := range pa.GetActions() {
		out[action] = int(count)
	}
	return out
}
