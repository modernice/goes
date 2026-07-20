# Lookups

`projection/lookup` is the built-in pattern for event-driven reverse lookups and uniqueness checks.

If you need to answer questions like "which aggregate owns this email?" or "is this slug already in use?", start here instead of building a custom map from scratch.

## When to use a lookup

Use `lookup.Lookup` when:

- the index is derived from events
- the index should rebuild itself on startup from the event store
- you need reverse lookups by a field other than aggregate UUID
- you want a reusable component that other services or handlers can depend on

Use a custom in-memory map when the data is private to one projection or handler and you do not need the lookup behavior as a reusable application component.

## The core pieces

The main types are:

- `lookup.Lookup` - the running lookup projection
- `lookup.Data` - implemented by event data that wants to update the lookup
- `lookup.Provider` - used inside `ProvideLookup(...)` to set or remove keys
- `lookup.Contains(...)` - convenience helper for "does this aggregate have a value for this key?"

The lookup listens to a set of events, applies any `lookup.Data` payloads it sees, and keeps an in-memory index up to date.

## Minimal example

Imagine a user aggregate that needs a unique email address:

```go
package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection/lookup"
)

const (
	UserAggregate = "auth.user"

	UserRegistered   = "auth.user.registered"
	UserEmailChanged = "auth.user.email_changed"
	UserDeleted      = "auth.user.deleted"

	UserEmail = "email"
)

type UserRegisteredData struct {
	Email string
}

func (data UserRegisteredData) ProvideLookup(p lookup.Provider) {
	p.Provide(UserEmail, strings.ToLower(data.Email))
}

type UserEmailChangedData struct {
	Email string
}

func (data UserEmailChangedData) ProvideLookup(p lookup.Provider) {
	p.Provide(UserEmail, strings.ToLower(data.Email))
}

type UserDeletedData struct{}

func (UserDeletedData) ProvideLookup(p lookup.Provider) {
	p.Remove(UserEmail)
}

type User struct {
	*aggregate.Base
	Email string
}

func NewUser(id uuid.UUID) *User {
	u := &User{Base: aggregate.New(UserAggregate, id)}

	event.ApplyWith(u, u.registered, UserRegistered)
	event.ApplyWith(u, u.emailChanged, UserEmailChanged)
	event.ApplyWith(u, u.deleted, UserDeleted)

	return u
}

func (u *User) Register(ctx context.Context, emails *lookup.Lookup, email string) error {
	email = strings.ToLower(email)

	if id, ok := emails.Reverse(ctx, UserAggregate, UserEmail, email); ok && id != u.AggregateID() {
		return errors.New("email already in use")
	}

	aggregate.Next(u, UserRegistered, UserRegisteredData{Email: email})
	return nil
}

func (u *User) registered(evt event.Of[UserRegisteredData]) {
	u.Email = evt.Data().Email
}

func (u *User) emailChanged(evt event.Of[UserEmailChangedData]) {
	u.Email = evt.Data().Email
}

func (u *User) deleted(event.Of[UserDeletedData]) {
	u.Email = ""
}
```

## Running the lookup

Create the lookup from the event store, event bus, and the event names that can update it:

```go
emails := lookup.New(store, bus, []string{
	UserRegistered,
	UserEmailChanged,
	UserDeleted,
})

errs, err := emails.Run(ctx)
if err != nil {
	return err
}
go logErrors(errs)

<-emails.Ready()
```

`Run(...)` starts a continuous projection internally. `Ready()` closes after the first catch-up job has been applied.

::: tip
`Lookup(...)`, `Reverse(...)`, and `Contains(...)` wait for readiness internally. Still, if your service should be fully warm before it serves requests, wait on `<-l.Ready()` during startup instead of letting the first request block.
:::

## Querying the lookup

Use `Reverse(...)` to find the aggregate by value:

```go
id, ok := emails.Reverse(ctx, UserAggregate, UserEmail, "alice@example.com")
```

Use `Lookup(...)` to fetch the value stored for a specific aggregate and key:

```go
value, ok := emails.Lookup(ctx, UserAggregate, UserEmail, userID)
```

Use `lookup.Contains(...)` when you only care whether a key exists for a specific aggregate:

```go
if lookup.Contains[string](ctx, emails, UserAggregate, UserEmail, userID) {
	// user currently has an indexed email
}
```

## Update and delete behavior

`Provide(...)` replaces the previous value for the same key before storing the new one. That makes update events simple: just provide the new value.

```go
func (data UserEmailChangedData) ProvideLookup(p lookup.Provider) {
	p.Provide(UserEmail, strings.ToLower(data.Email))
}
```

`Remove(...)` clears one or more keys for the current aggregate:

```go
func (UserDeletedData) ProvideLookup(p lookup.Provider) {
	p.Remove(UserEmail)
}
```

Two details matter in practice:

- reverse lookups only work for comparable values
- non-comparable values can still be stored and retrieved with `Lookup(...)`, but not indexed by `Reverse(...)`

## Advanced example: multi-key uniqueness

This pattern shows up often in application services: one aggregate needs multiple unique identifiers, all maintained from a single event stream.

```go
package catalog

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection/lookup"
)

const (
	PropertyAggregate  = "catalog.property"
	PropertyUpdated    = "catalog.property.updated"
	LookupSlug         = "slug"
	LookupInternalName = "internal_name"
)

type PropertyUpdatedData struct {
	Name         string
	InternalName string
}

func (data PropertyUpdatedData) ProvideLookup(p lookup.Provider) {
	p.Provide(LookupSlug, slug.Make(data.Name))
	p.Provide(LookupInternalName, data.InternalName)
}

type PropertyFactory struct {
	*lookup.Lookup
}

func NewPropertyFactory(store event.Store, bus event.Bus) *PropertyFactory {
	return &PropertyFactory{
		Lookup: lookup.New(store, bus, []string{PropertyUpdated}),
	}
}

func (pf *PropertyFactory) TestNameUpdate(propertyID uuid.UUID, name, internalName string) error {
	<-pf.Ready()

	if id, ok := pf.Reverse(context.Background(), PropertyAggregate, LookupSlug, slug.Make(name)); ok && id != propertyID {
		return fmt.Errorf("slug already in use")
	}

	if id, ok := pf.Reverse(context.Background(), PropertyAggregate, LookupInternalName, internalName); ok && id != propertyID {
		return fmt.Errorf("internal name already in use")
	}

	return nil
}
```

This is a good fit for `lookup.Lookup` because:

- the rules are derived from events, not an external SQL uniqueness constraint
- the lookup is reusable from multiple workflows
- startup replay reconstructs the index without a custom bootstrap path

## Lookup vs custom event handler

You can implement the same behavior with [`event/handler`](/guide/event-handlers), but `lookup.Lookup` should be your default when the component is fundamentally a reverse index.

Choose `lookup.Lookup` when:

- you want keyed lookups by aggregate ID and reverse lookups by value
- you want update and removal semantics packaged for you
- you want a small, reusable abstraction that other code can depend on

Choose a custom event handler when:

- the state shape is not really a key/value lookup
- you need custom in-memory structures or side effects
- the component is more observer than index

## Next steps

- For general read-model patterns, see [Projections](/guide/projections).
- For lower-level in-memory observers and caches, see [Event Handlers](/guide/event-handlers).
