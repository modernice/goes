package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/pick"
	"github.com/modernice/goes/projection"
)

// Permissions is the read-model for the permissions of a specific actor.
// Permissions uses the actor and role events to project the permissions of a
// specific actor. An actor is allowed to perform a given action if either the
// actor itself was granted the permission, or if the actor is a member of a
// role that was granted the permission.
//
// In order to fully remove a permission of an actor, the permission needs to
// be revoked from the Actor itself and also from all roles the actor is a
// member of (or the actor must be removed from these roles).
//
// For example, if an actor is a member of an "admin" role, and the following
// permissions are granted and revoked in the following order:
//	1. Actor is granted "view" permission on a "foo" aggregate.
//	2. Role is granted "view" permission on the same "foo" aggregate.
//	3. Role is revoked "view" permission on the aggregate.
// Then the actor is still allowed to perform the "view" action on the aggregate.
//
// Another example:
//	1. Role is granted "view" permission on a "foo" aggregate.
//  2. Actor is granted "view" permission on the same aggregate.
//  2. Actor is revoked "view" permission on the aggregate.
// Then the actor is also allowed to perform the "view" action on the aggregate
// because the role still grants the permission its members.
type Permissions struct {
	*projection.Base
	*projection.Progressor
	PermissionsDTO

	rolesHaveChanged bool
}

// PermissionsDTO is the DTO of Permissions.
type PermissionsDTO struct {
	ActorID uuid.UUID   `json:"actorId" bson:"actorId"`
	Roles   []uuid.UUID `json:"roles" bson:"roles"`
	OfActor Actions     `json:"ofActor" bson:"ofActor"`
	OfRoles Actions     `json:"ofRoles" bson:"ofRoles"`
}

// PermissionsOf returns the permissions read-model of the given actor.
// The returned projection has an empty state. A *Projector can be used to
// continuously project the permission read-models for all actors. Use a
// PermissionRepository to fetch the projected permissions of an actor:
//	var repo auth.PermissionRepository
//	var actorID uuid.UUID
//	perms, err := repo.Fetch(context.TODO(), actorID)
//	// handle err
//	allowed := perms.Allows("<action>", aggregate.Ref{Name: "...", ID: uuid.UUID{...}})
//	disallowed := perms.Disallows("<action>", aggregate.Ref{Name: "...", ID: uuid.UUID{...}})
func PermissionsOf(actorID uuid.UUID) *Permissions {
	perms := &Permissions{
		Base:       projection.New(),
		Progressor: projection.NewProgressor(),
		PermissionsDTO: PermissionsDTO{
			ActorID: actorID,
			OfActor: make(Actions),
			OfRoles: make(Actions),
		},
	}

	event.ApplyWith(perms, perms.granted, PermissionGranted)
	event.ApplyWith(perms, perms.revoked, PermissionRevoked)
	event.ApplyWith(perms, perms.roleGiven, RoleGiven)
	event.ApplyWith(perms, perms.roleRemoved, RoleRemoved)

	return perms
}

// ModelID returns the aggregate id of the actor. ModelID implements goes/persistence/model.Model.
func (perms PermissionsDTO) ModelID() uuid.UUID {
	return perms.ActorID
}

// Allows returns whether the actor is allowed to perform the given action on
// the given aggregate. An actor is allowed to perform a given action if either
// the actor itself was granted the permission, or if the actor is a member of a
// role that was granted the permission.
//
// Read the documentation of Permissions for more details.
func (perms PermissionsDTO) Allows(action string, ref aggregate.Ref) bool {
	return perms.ActorAllows(action, ref) || perms.RoleAllows(action, ref)
}

// ActorAllows returns whether the actor is allowed to perform the given action
// on the given aggregate, ignoring permissions of any roles the actor is member of.
func (perms PermissionsDTO) ActorAllows(action string, ref aggregate.Ref) bool {
	return perms.OfActor.allows(action, ref)
}

// RoleAllows returns whether the actor is allowed to perform the given action
// on the given aggregate, using only the permissions of the roles the actor is
// member of.
func (perms PermissionsDTO) RoleAllows(action string, ref aggregate.Ref) bool {
	return perms.OfRoles.allows(action, ref)
}

// Disallows returns whether the actor is disallows to perform the given action
// on the given aggregate. Disallows simply returns !perms.Allows(action, ref).
//
// Read the documentation of Permissions for more details.
func (perms PermissionsDTO) Disallows(action string, ref aggregate.Ref) bool {
	return !perms.Allows(action, ref)
}

// Equal returns whether perms and other contain exactly the same values.
func (perms PermissionsDTO) Equal(other PermissionsDTO) bool {
	return perms.ActorID == other.ActorID &&
		perms.OfActor.Equal(other.OfActor) &&
		perms.OfRoles.Equal(other.OfRoles)
}

func (perms *Permissions) granted(evt event.Of[PermissionGrantedData]) {
	switch pick.AggregateName(evt) {
	case ActorAggregate:
		perms.OfActor.granted(evt)
	case RoleAggregate:
		perms.OfRoles.granted(evt)
	}
}

func (perms *Permissions) revoked(evt event.Of[PermissionRevokedData]) {
	switch pick.AggregateName(evt) {
	case ActorAggregate:
		perms.OfActor.revoked(evt)
	case RoleAggregate:
		perms.OfRoles.revoked(evt)
	}
}

func (perms *Permissions) roleGiven(evt event.Of[[]uuid.UUID]) {
	perms.Roles = append(perms.Roles, pick.AggregateID(evt))
	perms.rolesHaveChanged = true
}

func (perms *Permissions) roleRemoved(evt event.Of[[]uuid.UUID]) {
	roleID := pick.AggregateID(evt)
	for i, role := range perms.Roles {
		if roleID == role {
			perms.Roles = append(perms.Roles[:i], perms.Roles[i+1:]...)
			perms.rolesHaveChanged = true
			return
		}
	}
}

func (perms *Permissions) finalize(ctx context.Context, roles RoleRepository) error {
	if !perms.rolesHaveChanged {
		return nil
	}
	perms.rolesHaveChanged = false
	perms.OfRoles = make(Actions)

	for _, roleID := range perms.Roles {
		role, err := roles.Fetch(ctx, roleID)
		if err != nil {
			return fmt.Errorf("fetch role: %w [id=%v]", err, roleID)
		}

		for target, actions := range role.Actions {
			for action := range actions {
				tactions, ok := perms.OfRoles[target]
				if !ok {
					tactions = make(map[string]int)
					perms.OfRoles[target] = tactions
				}
				tactions[action]++
			}
		}
	}

	return nil
}
