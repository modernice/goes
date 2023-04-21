package auth

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
)

// ActorIdentified is an event constant that represents the identification of an
// actor within the authentication system. It is used as a key for registering
// the event data type ActorIdentifiedData in a codec registry.
const (
	ActorIdentified = "goes.contrib.auth.actor.identified"

	RoleIdentified = "goes.contrib.auth.role.identified"
	RoleGiven      = "goes.contrib.auth.role.given"
	RoleRemoved    = "goes.contrib.auth.role.removed"

	// Permission events are used by both the Permission and Role aggregate.
	PermissionGranted = "goes.contrib.auth.permission_granted"
	PermissionRevoked = "goes.contrib.auth.permission_revoked"
)

// ActorIdentifiedData is the event data for ActorIdentified.
type ActorIdentifiedData string

// RoleIdentifiedData is the event data for RoleIdentified.
type RoleIdentifiedData string

// PermissionGrantedData is the event data for PermissionGranted.
type PermissionGrantedData struct {
	Aggregate aggregate.Ref
	Actions   []string
}

// PermissionRevokedData is the event data for PermissionRevoked.
type PermissionRevokedData struct {
	Aggregate aggregate.Ref
	Actions   []string
}

// RegisterEvents registers the events of the auth package into a registry.
func RegisterEvents(r codec.Registerer) {
	codec.Register[ActorIdentifiedData](r, ActorIdentified)
	codec.Register[RoleIdentifiedData](r, RoleIdentified)
	codec.Register[[]uuid.UUID](r, RoleGiven)
	codec.Register[[]uuid.UUID](r, RoleRemoved)
	codec.Register[PermissionGrantedData](r, PermissionGranted)
	codec.Register[PermissionRevokedData](r, PermissionRevoked)
}
