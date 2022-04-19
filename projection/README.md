# Projections

Package `projection` provides a framework for building and managing projections.

## Introduction

A projection is any type that implements the `Target` interface. A projection
target can apply events to itself to _project its state._

```go
package projection

type Target interface {
	ApplyEvent(event.Event)
}
```

To build the projection state, you can use the `Apply()` function provided by
this package. Each of the provided events will be applied to the target:

```go
package example

import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/projection"
)

func example(target projection.Target, events []event.Event) {
	projection.Apply(target, events)
}
```

The `Apply()` function also supports the following optional APIs:

- [`ProgressAware`](#progressaware)
- [`Guard`](#guard)

### Example – Lookup table

This example shows how to implement a lookup table for email addresses of
registered users. Each time a `"user_registered"` event occurs, the lookup table
is updated:

```go
package example

import (
	"sync"
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/helper/pick"
)

// Emails is a thread-safe lookup table for email addresses <> user ids.
type Emails struct {
	mux sync.RWMutex
	users map[string]uuid.UUID // map[EMAIL]USER_ID
	emails map[uuid.UUID]string // map[USER_ID]EMAIL
}

// NewEmails returns the lookup table for email addresses <> user ids.
func NewEmails() *Emails {
	return &Emails{
		users: make(map[string]uuid.UUID),
		emails: make(map[uuid.UUID]string),
	}
}

// UserID returns the id of the user with the given email address.
func (emails *Emails) UserID(email string) (uuid.UUID, bool) {
	emails.mux.RLock()
	defer emails.mux.RUnlock()
	id, ok := emails.users[email]
	return id, ok
}

// Email returns the email address of the user with the given id.
func (emails *Emails) Email(userID uuid.UUID) (string, bool) {
	emails.mux.RLock()
	defer emails.mux.RUnlock()
	email, ok := emails.emails[userID]
	return email, ok
}

// ApplyEvent implements projection.Target.
func (emails *Emails) ApplyEvent(evt event.Event) {
	emails.mux.Lock()
	defer emails.mux.Unlock()

	switch evt.Name() {
	case "user_registered":
		userID := pick.AggregateID(evt)
		email := evt.Data().(string)
		emails.users[email] = userID
		emails.emails[userID] = email
	case "user_deleted":
		userID := pick.AggregateID(evt)
		if email, ok := emails.emails[userID]; ok {
			delete(emails.users, email)
		}
		delete(emails.emails, userID)
	}
}
```

## Scheduling

Given the example above, the lookup table would never be automatically populated.
Typically, you want a projection to be updated with every published event within
a specified set of events. Using the lookup table as an example, it should be
updated on every published `"user_registered"` and `"user_deleted"` event.
This can be achieved using a [continuous schedule](#continuous).

### Continuous

The continuous schedule subscribes to events over an event bus to trigger
[projection jobs](#projection-jobs) when events of a specified set are
published. The projection job can be applied to a projection to update its
state.

```go
package example

// ... previous code ...

import (
	"context"
	"github.com/modernice/goes/projection/schedule"
)

func example(bus event.Bus, store event.Store) {
	emails := NewEmails()

	s := schedule.Continuously(bus, store, []string{
		"user_registered",
		"user_deleted",
	})

	errs, err := s.Subscribe(context.TODO(), func(ctx projection.Job) error {
		// Apply the projection job to the projection.
		return ctx.Apply(ctx, emails)
	})

	if err != nil {
		panic(fmt.Errorf("subscribe to projection schedule: %w", err))
	}

	for err := range errs {
		log.Printf("failed to apply projection: %v", err)
	}
}
```

#### Debounce

If your application publishes a lot of events in a short amount of time,
consider providing the `Debounce(time.Duration)` option to continuous schedules
to improve the performance of your application.

Each time one of the configured events is published, the schedule will wait for
the specified duration before triggering a projection job. Additional events
that are published during this time will be buffered by the schedule and passed
as a unit to the triggered projection job. Should an event be published and
buffered during this wait time, the wait timer resets.

> TODO: Implement a wait cap.

```go
package example

func example(bus event.Bus, store event.Store) {
	s := schedule.Continuously(
		bus, store, []string{"..."},

		// Debounce projection jobs by 1 second.
		schedule.Debounce(time.Second),
	)
}
```

### Periodic

A periodic schedule triggers [projection jobs](#projection-jobs) at a
specified interval. Periodic schedules always fetch the entire history of the
configured events from the event store to apply to the projections.

```go
package example

// ... previous code ...

func example(store event.Store) {
	emails := NewEmails()

	// Trigger a projection job every hour.
	s := schedule.Periodically(store, time.Hour, []string{
		"user_registered",
		"user_deleted",
	})

	errs, err := s.Subscribe(context.TODO(), func(ctx projectio.Job) error {
		return ctx.Apply(ctx, emails)
	})

	if err != nil {
		panic(fmt.Errorf("subscribe to projection schedule: %w", err))
	}

	for err := range errs {
		log.Printf("failed to apply projection: %v", err)
	}
}
```

## Projection jobs

Jobs are typically created by schedules when triggering a projection update.
A job can be applied to projections to update their state by applying the
events that are configured in the job. Depending on the schedule that triggered
the job, the job may fetch events on-the-fly from the event store when applied
onto a projection.

A job provides additional query helpers to extract event and aggregate
information from the events in the job. All query functions of a `Job` use
caching to avoid querying the underlying event stream unnecessarily.
Jobs are thread-safe, which means that they can be applied concurrently onto
multiple projections.

```go
package example

// ... previous code ...

func example(s projection.Schedule) {
	emails := NewEmails()

  errs, err := s.Subscribe(context.TODO(), func(ctx projection.Job) error {
    // Query all events of the job.
    events, errs, err := ctx.Events(ctx)

    // Query all events of the job that belong to one of the given aggregate names.
    events, errs, err := ctx.EventsOf(ctx, "user")

    // Query all events of the job that would be applied to the given projection.
    events, errs, err := ctx.EventsFor(ctx, emails)

    // Extract all aggregates from the job's events as aggregate.Refs.
    refs, errs, err := ctx.Aggregates(ctx)

    // Extract all aggregates with one of the given names from the job's events
    // as aggregate.Refs.
    refs, errs, err := ctx.Aggregates(ctx, "user")

    // Extract the first UUID of the aggregate with the given name from the events
    // of the job.
    id, err := ctx.Aggregate(ctx, "user")

    return nil
  })
  if err != nil {
    log.Fatalf("subscribe to projection schedule: %v", err)
  }

  for err := range errs {
    log.Printf("failed to project: %v", err)
  }
}
```

### Startup / initial projections

When subscribing to a schedule, you can provide the `Startup(TriggerOption)`
option to trigger an initial projection update on startup.

```go
package example

func example(s projection.Schedule) {
	errs, err := s.Subscribe(
		context.TODO(),
		func(projection.Job) error { ... },

		// immediately create and apply a projection job
		projection.Startup(), 
	)

	if err != nil {
		// initial projection job failed
	}

	for err := range errs {
		log.Printf("subsequent projection job failed: %v", err)
	}
}
```

### Manually trigger a job

Both continuous and periodic schedules can be manually triggered at any time
using the `Schedule.Trigger()` method.

When triggering a continuous schedule, if no custom event query is provided to
the trigger, the entire history of the configured events is fetched from the
event store, just like it's done within periodic schedules.

```go
package example

func example(s projection.Schedule) {
	if err := s.Trigger(context.TODO()); err != nil {
		panic(fmt.Error("failed to trigger projection: %w", err))
	}
}
```

You can override the query that is used by the job's `Aggregates()` helper to
improve its query performance:

```go
package example

func example(s projection.Schedule) {
	s.Subscribe(context.TODO(), func(ctx projection.Job) error {
		// We want to modify the underlying event query of this call.
		refs, errs, err := ctx.Aggregates(ctx)
	})

	s.Trigger(context.TODO(), projection.AggregateQuery(query.New(
		// query "foo" and "bar" events from which the aggregate data
		// will be extracted
		query.Name("foo", "bar"),
	)))
}
```

Alternatively, you can use the `NewJob()` constructor to create a job manually
without using a schedule:

```go
package example

// ... previous code ...

func example(store event.Store) {
	emails := NewEmails()

	job := projection.NewJob(context.TODO(), store, query.New(
		query.Name("user_registered", "user_deleted"),
	))

	if err := job.Apply(context.TODO(), emails); err != nil {
		panic(fmt.Errorf("apply projection job: %w", err))
	}
}
```

## Extensions

### ProgressAware

You can embed the `*Progressor` type into your projection to make the
projection `ProgressAware`. Such a projection keeps track of its "projection
progress" when updated, which

1. guards the projection from old, already applied events
2. can optimize the query performance of projection jobs by only querying
	 events that occured after a projection's progress time
3. allows for simple and performant
	 [startup projection jobs](#startup-projection-jobs)

```go
package example

type Foo struct {
	*projection.Progressor
}

func NewFoo() *Foo {
	return &Foo{
		Progressor: projection.NewProgressor(),
	}
}
```

### Guard

If a projection implements `Guard`, its `GuardProjection(event.Event)` is called
for every event that is about to be applied to the projection, and is only
applied if `GuardProjection()` returns `true`.

#### Example – Ecommerce orders

Given an ecommerce app where read models of orders need to be projected for the
customers. For brevity, the read model just needs to provide the id and total
price of the order. One could implement a projector for the order read models
like this:

```go
package example

// ... order aggregate implementation ..

// OrderPlaced is the event data for the "order_placed" event.
type OrderPlaced struct {
	UnitPrice int64
	Quantity int
}

// CustomerOrder is the read model of an order for the customer of the order.
type CustomerOrder struct {
	ID uuid.UUID
	Total int64
}

// NewCustomerOrder returns the customer read model of the order with the given id.
func NewCustomerOrder(id uuid.UUID) *CustomerOrder {
	return &CustomerOrder{
		ID: id,
	}
}

// GuardProjection implements projection.Guard.
func (order *CustomerOrder) GuardProjection(evt event.Event) bool {
	// Only allow events of the order that this read model represents.
	return pick.AggregateID(evt) == order.ID
}

// ApplyEvent implements projection.Target.
func (order *CustomerOrder) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case "order_placed":
		data := order.Data().(OrderPlaced)
		order.Total += data.UnitPrice * data.Quantity
	}
}

// ProjectCustomerOrders continuously projects order read models for
// customers until ctx is canceled.
func ProjectCustomerOrders(
	ctx context.Context,
	bus event.Bus,
	store event.Store,
) (<-chan error, error) {
	// Create a schedule that is triggered by the "order_placed" event.
	// We debounce the schedule by 1 second which can trigger a single
	// projection job for events of multiple orders.
	s := schedule.Continuously(
		bus, store, []string{"order_placed"},
		schedule.Debounce(time.Second),
	)

	return s.Subscribe(ctx, func(ctx projection.Job) error {
		// Extract the the orders from the events.
		refs, errs, err := ctx.Aggregates(ctx)
		if err != nil {
			return fmt.Error("extract aggregates: %w", err)
		}

		// For each order, create (or fetch) the read model and apply
		// the job to it.
		return streams.Walk(ctx, func(ref aggregate.Ref) error {
			order := NewCustomerOrder(ref.ID) // or fetch it from a repository

			// Simply apply the job to the projection and let the projection
			// guard determine which events are actually applied.
			return ctx.Apply(ctx, order)
		}, refs, errs)
	})
}
```

The projection guard example above could also be rewritten using the
`QueryGuard` provided by this package:

```go
package example

type CustomerOrder struct {
	projection.Guard

	ID uuid.UUID
	Total int64
}

func NewCustomerOrder(id uuid.UUID) *CustomerOrder {
	return &CustomerOrder{
		// Use an event query as the projection guard.
		Guard: projection.QueryGuard(query.New(query.AggregateID(id))),
		ID: id,
	}
}
```

## Projection service

The projection service allows projections to be triggered from external services
/ processes. Communication between projection services is done using events.

```go
package service1

func example(reg *codec.Registry, bus event.Bus) {
  // Register the events of the projection service into a registry.
  projection.RegisterService(reg)

  svc := projection.NewService(bus)

  // Given some named schedules
  var schedules map[string]projection.Schedule

  // When registering them in the projection service
  for name, s := range schedules {
    svc.Register(name, s)
  }
}

package service2

func example(bus event.Bus) {
  svc := projection.NewService(bus)

  // Then another service that uses the same underlying event bus can
  // trigger the registered schedules
  err := svc.Trigger(context.TODO(), "foo")
}
```


## Generic helpers

Applying events within the `ApplyEvent` function is the most straightforward way
to implement a projection but can become quite messy if a projection depends on
many events.

goes provides type-safe, generic helpers that allow you to setup an event
applier function for each individual event. This is what the lookup example
looks like using generics:

```go
package example

type Emails struct {
	*projection.Base // implements convenience methods

	mux sync.RWMutex
	users map[string]uuid.UUID // map[EMAIL]USER_ID
	emails map[uuid.UUID]string // map[USER_ID]EMAIL
}

func NewEmails() *Emails {
	emails := &Emails{Base: projection.New()}

	event.ApplyWith(emails, emails.userRegistered, "user_registered")
	event.ApplyWith(emails, emails.userDeleted, "user_deleted")

	return emails
}

func (emails *Emails) userRegistered(evt event.Of[string]) {
	emails.mux.Lock()
	defer emails.mux.Unlock()

	userID := pick.AggregateID(evt)
	email := evt.Data().(string)
	emails.users[email] = userID
	emails.emails[userID] = email
}

func (emails *Emails) userDeleted(evt event.Event) {
	emails.mux.Lock()
	defer emails.mux.Unlock()

	userID := pick.AggregateID(evt)
	if email, ok := emails.emails[userID]; ok {
		delete(emails.users, email)
	}
	delete(emails.emails, userID)
}
```

## Tips

### Startup projection jobs

Consider making your projection [`ProgressAware`](#progressaware) if you want to
use the `Startup()` option when subscribing to a schedule. This can hugely
improve the query performance of the initial projection job because the job can
optimize its queries using the progress time provided by the `ProgressAware`
interface. If your projection does not implement `ProgressAware`, then the
initial projection job will query the entire history of the configured events,
which – _depending on the size of your event store_ – could take a long time.

### Projection finalization

Avoid long-running function calls in the event appliers. Move such calls to a
finalizer method on your projection and call it after applying the job, like this:

```go
package example

type Foo struct { ... }

func (*Foo) ApplyEvent(event.Event) {}

func (f *Foo) finalize(ctx context.Context, dep SomeDependency) error {
	// do stuff
	if err := dep.Do(ctx, "..."); err != nil {
		return err
	}
	// do more stuff
	return nil
}

func example(s projection.Schedule) {
	var foo Foo
	var dep SomeDependency

	s.Subscribe(context.TODO(), func(ctx projection.Job) error {
		if err := ctx.Apply(ctx, &foo); err != nil {
			return err
		}

		return foo.finalize(ctx, dep)
	})
}
```

> TODO: Implement finalization queue to allow for batch finalization.
