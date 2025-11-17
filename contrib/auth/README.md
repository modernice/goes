# Authorization Module

Package `auth` implements a ready-to-use authorization system for goes-driven apps.

This module implements an authorization system around the concepts of **actors
and roles,** and builds on top of goes' aggregate, event, and command system.

User management is explicitly **not** provided by this module. Instead, it
allows you to integrate your own or third-party user management using *custom actors.*

## Features

- Role-based authorization
- Action-based authorization
- Aggregate-specific authorization
- Wildcard support

## Design

### Actors

An `Actor` represents a user within the system, which can be anything, from a
real-world human to a system user to an API key. Actors are aggregates, and by
default, are simply identified by their aggregate ids (UUID).

Another actor kind provided by this module is the string-Actor, which is not
only identified by its aggregate id but also by some arbitrary `string`. For
example, the user of an API key can be granted permission to perform actions
through the string-Actor aggregate.

An actor can be granted an arbitrary amount of permissions. All permissions can
be revoked from actors after they were granted.

#### Example

```go
package example

// Grant a UUID-Actor the permission to "view" and "update" a "foo" aggregate.
func exampleUUIDActor() {
	actor := auth.NewUUIDActor(internal.NewUUID())

	actor.Grant(aggregate.Ref{
		Name: "foo",
		ID: uuid.UUID{...},
	}, "view", "update")
}

// Grant a string-Actor the permission to "view" and "update" a "foo" aggregate.
func exampleStringActor() {
	actor := auth.NewStringActor(internal.NewUUID())

	actor.Identify("foo-bar-baz")

	actor.Grant(aggregate.Ref{
		Name: "foo",
		ID: uuid.UUID{...},
	}, "view", "update")
}
```

### Roles

A `Role` represents a group of actors that are granted permissions to perform
specific actions on specific aggregates. Actors are allowed to perform an action
if they were either granted the permission directly or if they are a member of a
role that was granted the permission.

```go
package example

// Grant a role the permission to "view" and "update" a "foo" aggregate.
func example() {
	role := auth.NewRole(internal.NewUUID())

	role.Identify("admin")

	role.Grant(aggregate.Ref{
		Name: "foo",
		ID: uuid.UUID{...},
	}, "view", "update")
}
```

### Permissions

The actual permissions of an actor cannot be queried from the `Actor` aggregate
alone because an actor may have permissions that were granted to them through a
role. In order to query the permission of an actor, the `Permissions` read-model
must be used instead. The permission read-model projects the actor and role
events to provide the actual permissions of an actor.

```go
package example

func example(permissions auth.PermissionRepository, actorID uuid.UUID) {
	perms, err := permissions.Fetch(context.TODO(), actorID)
	// handle err

	canView := perms.Allows("view", aggregate.Ref{
		Name: "foo",
		ID: uuid.UUID{...},
	})
}
```

## Usage

### Setup

The authorization module can be integrted into an existing goes application:

```go
package main

func main() {
	// Setup of your own application.
	var eventRegistry *codec.Registry
	var commandRegistry *codec.Registry
	var bus event.Bus
	var store event.Store
	var repo aggregate.Repository
	var cbus command.Bus

	// Authorization setup

	// Register "auth" events and commands into your registries.
	auth.RegisterEvents(eventRegistry)
	auth.RegisterCommands(commandRegistry)

	// Create "auth" repositories
	actorRepos := auth.NewActorRepositories()
	roles := auth.NewRoleRepository(repo)
	permissions := auth.InMemoryPermissionRepository()

	// Run the lookup projection in the background.
	lookup := auth.NewLookup(store, bus)
	lookupErrors, err := lookup.Run(context.TODO())
	// handle err

	// Run the permission projector in the background.
	permissionProjector := auth.NewPermissionProjector(permissions, bus, store)
	permissionErrors, err := permissionProjector.Run(context.TODO())
	// handle err

	// Handle "auth" commands in the background.
	authErrors, err := auth.HandleCommads(context.TODO(), cbus, actorRepos, roles)
	// handle err

	errs := streams.FanInAll(lookupErrors, permissionErrors, authErrors)

	// Log errors.
	for err := range errs {
		log.Println(err)
	}
}
```

### Grant Permissions

Permissions can be granted to actors and roles. The following example grants
the permissions to "view" and "update" the "foo" aggregate with the given
`fooID` to the provided actor and role.

```go
package example

func example(actor *auth.Actor, role *auth.Role, fooID uuid.UUID) {
	actor.Grant(aggregate.Ref{
		Name: "foo",
		ID: fooID,
	}, "view", "update")

	role.Grant(aggregate.Ref{
		Name: "foo",
		ID: fooID,
	}, "view", "update")
}

// or using the command system
func example(bus command.Bus, actorID, roleID, fooID uuid.UUID) {
	cmd := auth.GrantToActor(actorID, aggregate.Ref{...}, "view", "update")
	bus.Dispatch(context.TODO(), cmd)

	cmd := auth.GrantToRole(roleID, aggregate.Ref{...}, "view", "update")
	bus.Dispatch(context.TODO(), cmd)
}
```

### Revoke Permissions

Permissions can also be revoked from actors and roles. The following example
revokes the permissions to "view" and "update" the "foo" aggregate with the
given `fooID` from the provided actor and role.

```go
package example

func example(actor *auth.Actor, role *auth.Role, fooID uuid.UUID) {
	actor.Revoke(aggregate.Ref{
		Name: "foo",
		ID: fooID,
	}, "view", "update")

	role.Revoke(aggregate.Ref{
		Name: "foo",
		ID: fooID,
	}, "view", "update")
}

// or using the command system
func example(bus command.Bus, actorID, roleID, fooID uuid.UUID) {
	cmd := auth.RevokeFromActor(actorID, aggregate.Ref{...}, "view", "update")
	bus.Dispatch(context.TODO(), cmd)

	cmd := auth.RevokeFromRole(roleID, aggregate.Ref{...}, "view", "update")
	bus.Dispatch(context.TODO(), cmd)
}
```

### HTTP Middleware

The `http/middleware` package implements HTTP middleware that can be used to
protect routes from unauthorized access. The two main middlewares are:
- `Authorize(...)` – authorizes actors
- `Permission(...)` – protects routes

In order to protect a route, these two middlewares must be added to the HTTP
handler. The `Authorize()` middleware must be called before the `Permission()`
middleware.

A middleware `Factory` can be used to create middleware without having to pass
a `PermissionFetcher` or `Lookup` each time:

```go
package example

func example(perms middleware.PermissionFetcher, lookup *auth.Lookup) {
	factory := middleware.NewFactory(perms, lookup)

	authorize := factory.Authorize(func(middleware.Authorizer, *http.Request) {
		// ...
	})

	permission := factory.Permision("view", func(*http.Request) aggregate.Ref {
		// ...
	})
}
```

#### Example using [go-chi](https://github.com/go-chi/chi)

```go
package example

func example(factory middleware.Factory) {
	r := chi.NewRouter()
	r.Use(
		factory.Authorize(func(auth middleware.Authorizer, r *http.Request) {
			var actorID uuid.UUID // extract from request
			auth.Authorize(actorID) // authorize the given actor

			// multiple actors can be authorized at the same time
			auth.Authorize(<another-actor-id>)
		}),

		factory.Permission("view", func(r *http.Request) aggregate.Ref {
			// we must return which aggregate the actor wants to "view".
			return aggregate.Ref{
				Name: "foo",
				ID: chi.URLParam(r, "FooID"),
			}
		}),
	)
}
```

### Custom Actors

This module implements actors for two kinds of identifiers: UUIDs and strings.
These two kinds should suffice for most applications but if required, your
application may configure additional actor kinds.

#### Example

```go
package example

const CustomActorKind = "custom"

type ActorID struct {
	Foo string
	Bar int
}

func (id ActorID) String() string {
	return fmt.Sprintf("%s_%d", id.Foo, id.Bar)
}

// NewCustomActor is the constructor of your custom actor kind.
// The provided auth.ActorConfig provides the parser and formatter for the
// actor id.
func NewCustomActor(id uuid.UUID) *auth.Actor {
	return auth.NewActor(id, auth.ActorConfig[ActorID]{
		Kind: CustomActorKind,
		FormatID: func(id ActorID) string {
			return id.String()
		},
		ParseID: func(v string) (ActorID, error) {
			parts := strings.Split(v, "_")
			if len(parts) != 2 {
				return ActorID{}, errors.New("invalid id")
			}
 
			bar, err := strconv.Atoi(parts[1])
			if err != nil {
				return ActorID{}, fmt.Errorf("parse Bar: %w", err)
			}

			return ActorID{
				Foo: parts[0],
				Bar: bar,
			}, nil
		},
	})
}

// NewCustomActorRepository returns an ActorRepository that uses the
// NewCustomActor constructor to create a actors.
func NewCustomActorRepository(repo aggregate.Repository) auth.ActorRepository {
	return repository.Typed(repo, NewCustomActor)
}

func setup(repo aggregate.Repository) {
	actorRepos := auth.NewActorRepositories()

	customRepo := NewCustomActorRepository(repo)

	actorRepos.Add(CustomActorKind, customRepo)

	repo, _ := actorRepos.Repository(CustomActorKind)
	// repo == customRepo
}

func example(l *auth.Lookup, f middleware.Factory) {
	actor := NewCustomActor(internal.NewUUID())
	
	id := ActorID{
		Foo: "foo",
		Bar: 1837,
	}
	// Actors that are not identified by a UUID, must first be identified.
	actor.Identify(id)

	// After being identified, the Lookup maps the formatted id to the actual
	// aggregate id of the actor.
	aggregateID, ok := l.Actor(id.String())
	// ok == true
	// aggregateID == actor.AggregateID()

	// The lookup allows to retrieve the actual aggregate id of a non-UUID-Actor
	// from within the Authorize() middleware.
	mw := f.Authorize(func(auth middleware.Authorizer, r *http.Request) {
		// Extract the formatted custom actor id from the request. 
		var sid string

		// Then lookup the aggregate id (UUID) of that actor 
		id, ok := auth.Lookup(sid)
		// ok == true
		// id == actor.AggregateID()

		auth.Authorize(id)
	})
}
```

### Wildcards

Permissions can be added to actors and roles using wildcards. When a wildcard is
provided, it matches all possible values for the given field. Wildcard for
`string` fields is `*`, wildcard for `UUID` fields is `uuid.Nil`.

#### Example

```go
package example

func example(actor *auth.Actor) {
	// Grant "view" and "update" permission on all aggregates with a specific
	// aggregte id.
	actor.Grant(aggregate.Ref{
		Name: "*",
		ID: uuid.UUID{...},
	}, "view", "update")

	// Grant "view" and "update" permission on all "foo" aggregates.
	actor.Grant(aggregate.Ref{
		Name: "foo",
		ID: uuid.Nil,
	}, "view", "update")

	// Grant permission for all actions on a specific "foo" aggregate.
	actor.Grant(aggregate.Ref{
		Name: "foo",
		ID: uuid.UUID{...},
	}, "*")

	// Grant permission for all actions on all aggregates.
	actor.Grant(aggregate.Ref{
		Name: "*",
		ID: uuid.Nil,
	}, "*")
}
```

### gRPC Server

The `authrpc` package implements a gRPC server and client that can be used to
implement an authorization service. Other services can query the auth service
to check permissions of actors and to lookup actor ids.

Protobufs are defined in the [github.com/modernice/goes/api/proto](/api/proto) package.

```go
package example

import (
	authpb "github.com/modernice/goes/api/proto/gen/auth"
)

func server(perms auth.PermissionRepository) {
	s := grpc.NewServer()
	authpb.RegisterAuthServiceServer(s, authrpc.NewServer(perms))
}

func client(actorID uuid.UUID) {
	conn, err := grpc.Dial(...)
	// handle err

	client := authrpc.NewClient(conn)

	perms, err := client.Permissions(context.TODO(), actorID)
	// handle err

	perms.Allows(...)
	perms.Disallows(...)
}
```
