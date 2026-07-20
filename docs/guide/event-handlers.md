# Event Handlers

`event/handler` is the lightweight option for long-lived in-memory observers.

Use it when you want to subscribe to events, keep some process-local state warm, or run event-driven side effects without building a full projection model.

## When to use `event/handler`

`event/handler` is a good fit for:

- in-memory lookup tables and caches
- side-effect processors that react to events
- lightweight observers that should replay history on startup
- application components that do not need persistence, progress tracking, or projection scheduling

Use a [projection](/guide/projections) instead when you need:

- persisted read models
- `projection.Progressor`
- debounce or periodic schedules
- resettable or triggerable projection jobs

If the component is specifically a reverse lookup or uniqueness index, start with [Lookups](/guide/lookups).

## Minimal setup

Create a handler from an event bus, register typed handlers with `event.HandleWith(...)`, then run it.

```go
package audit

import (
	"sync/atomic"

	"github.com/modernice/goes/event"
	evhandler "github.com/modernice/goes/event/handler"
)

const UserRegistered = "auth.user.registered"

type UserRegisteredData struct {
	Email string
}

type Counter struct {
	*evhandler.Handler
	total atomic.Int64
}

func NewCounter(bus event.Bus) *Counter {
	c := &Counter{
		Handler: evhandler.New(bus),
	}

	event.HandleWith(c, c.userRegistered, UserRegistered)

	return c
}

func (c *Counter) Total() int64 {
	return c.total.Load()
}

func (c *Counter) userRegistered(event.Of[UserRegisteredData]) {
	c.total.Add(1)
}
```

Then run it:

```go
counter := NewCounter(bus)

errs, err := counter.Run(ctx)
if err != nil {
	return err
}
go logErrors(errs)
```

## Startup replay

By default, the handler only sees new events from the bus. Add `handler.Startup(store)` when the handler should rebuild state from the event store first.

```go
h := evhandler.New(bus, evhandler.Startup(store))
```

This is the key difference between a transient subscriber and an application component that can recover after a restart.

## Main example: profile lookup

This pattern is common in applications that need fast in-memory lookups derived from events.

```go
package catalog

import (
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	evhandler "github.com/modernice/goes/event/handler"
	"github.com/modernice/goes/helper/pick"
)

const ProfileCreated = "catalog.profile.created"

type ProfileCreatedData struct {
	PropertyID uuid.UUID
	Language   string
}

type ProfileLookup struct {
	*evhandler.Handler

	mux    sync.RWMutex
	lookup map[uuid.UUID]map[string]uuid.UUID
}

func NewProfileLookup(bus event.Bus, store event.Store) *ProfileLookup {
	pl := &ProfileLookup{
		Handler: evhandler.New(bus, evhandler.Startup(store)),
		lookup:  make(map[uuid.UUID]map[string]uuid.UUID),
	}

	event.HandleWith(pl, pl.profileCreated, ProfileCreated)

	return pl
}

func (pl *ProfileLookup) Profile(propertyID uuid.UUID, language string) (uuid.UUID, bool) {
	pl.mux.RLock()
	defer pl.mux.RUnlock()

	profiles, ok := pl.lookup[propertyID]
	if !ok {
		return uuid.Nil, false
	}

	id, ok := profiles[language]
	return id, ok
}

func (pl *ProfileLookup) profileCreated(evt event.Of[ProfileCreatedData]) {
	pl.mux.Lock()
	defer pl.mux.Unlock()

	data := evt.Data()
	profiles, ok := pl.lookup[data.PropertyID]
	if !ok {
		profiles = make(map[string]uuid.UUID)
		pl.lookup[data.PropertyID] = profiles
	}

	profiles[data.Language] = pick.AggregateID(evt)
}
```

Why this is a good `event/handler` use case:

- the state is process-local and in-memory
- rebuilding from history is enough; no separate persistence layer is needed
- the component wants typed event callbacks, not a scheduled projection pipeline

## `Startup(...)` vs `StartupQuery(...)`

`Startup(store, opts...)` is the simple option. It tells the handler to query the store before processing new events.

```go
import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
)

h := evhandler.New(bus, evhandler.Startup(
	store,
	query.Name(ProfileCreated),
	query.SortByTime(),
))
```

When you pass query options directly to `Startup(...)`, they replace the default startup query.

Use `StartupQuery(...)` when you want to modify the default query instead:

```go
h := evhandler.New(
	bus,
	evhandler.Startup(store),
	evhandler.StartupQuery(func(q event.Query) event.Query {
		return query.Merge(q, query.New(query.AggregateName("catalog.profile")))
	}),
)
```

The rule of thumb:

- `Startup(store, ...)` when you know the full startup query you want
- `StartupQuery(...)` when you want to tweak the default query shape

## Workers

Handlers process events with one worker by default. Increase this with `handler.Workers(n)` when event callbacks are independent and your state is safe for concurrent access.

```go
h := evhandler.New(
	bus,
	evhandler.Startup(store),
	evhandler.Workers(4),
)
```

Use multiple workers carefully. If handlers mutate shared maps or slices, protect them with a mutex or keep the worker count at `1`.

## How this differs from projections

`event/handler` gives you:

- direct bus subscription
- typed event callbacks
- optional startup replay
- simple process-local state

Projections give you:

- scheduled jobs
- `Apply(...)`, `Aggregates(...)`, and other job helpers
- debounce and periodic execution
- progress tracking and reset semantics

If you catch yourself needing persistent progress, batch jobs, or projection orchestration, move the component to [Projections](/guide/projections).
