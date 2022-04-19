# Projections

Package `projection` provides a framework for building and managing projections.

## Introduction

A projection is any type that implements the `Target` interface. A projection
target can apply events onto itself to _project its state._

```go
package projection

type Target interface {
	ApplyEvent(event.Event)
}
```

To build the projection state, you can use the `Apply()` function provided by
this package. Each of the provided events will be applied onto the target:

```go
package example

func example(target projection.Target, events []event.Event) {
	projection.Apply(target, events)
}
```

The `Apply()` function also supports the following optional APIs:

- [`ProgressAware`](#progress-aware)
- [`Guard`](#guards)

### Example – Lookup table

This example shows how to implement a lookup table for email addresses of
registered users. Each time a `"user_registered"` event occurs, the lookup table
is updated:

```go
package example

import (
	"sync"
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
published. The projection job can be applied onto a projection to update its
state.

```go
package example

// ... previous code ...

func example(bus event.Bus, store event.Store) {
	emails := NewEmails()

	s := schedule.Continuously(bus, store, []string{
		"user_registered",
		"user_deleted",
	})

	errs, err := s.Subscribe(context.TODO(), func(ctx projection.Job) error {
		// Apply the projection job onto the projection.
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

### Periodic

A periodic schedule triggers [projection jobs](#projection-jobs) at a
specified interval. Periodic schedules always fetch the entire history of the
configured events from the event store to apply onto the projections.

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
A job can be applied onto projections to update their state by applying the
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

    // Query all events of the job that would be applied onto the given projection.
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
