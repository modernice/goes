# Projections

## Schedules

A `Schedule` defines when projection `Jobs` should be created and passed to
subscribers of a Schedule. Schedules can be triggered manually.

### Create schedule

```go
package example

// create a Schedule that triggers continuously on every given Event
func createContinuousSchedule(bus event.Bus, store event.Store) projection.Schedule {
	return schedule.Continuously(ebus, estore, []string{"foo", "bar", "baz"})
}

// create a Schedule that triggers periodically and uses the provided Query
// builder to query Events
func createPeriodicSchedule(store event.Store, interval time.Duration) projection.Schedule {
	return schedule.Periodically(estore, interval, func(ctx schedule.PeriodicContext) event.Query {
		interval := ctx.Interval()
		triggeredAt := ctx.TriggerTime()

		return query.New(
			query.AggregateName("foo"),
			// ...
		)
	})
}
```

### Subscribe to schedule

```go
package example

type exampleProjection struct {}

func newExampleProjection() *exampleProjection {
	return &exampleProjection{}
}

func (*exampleProjection) ApplyEvent(event.Event) {}

func subscribeToSchedule(ctx context.Context, s projection.Schedule) (<-chan error, error) {
	return s.Subscribe(ctx, func(job projection.Job) error {
		proj := newExampleProjection()

		// Fetch all Events
		events, errs, err := job.Events(job.Context())

		// Fetch Events with additional filter
		events, errs, err := job.Events(job.Context(), query.New(...))

		// Fetch Events of Aggregates with given names
		events, errs, err := job.EventsOf(job.Context(), "foo", "bar", "baz")

		// Fetch Events for a given Target
		events, errs, err := job.EventsFor(job.Context(), proj)

		// Fetch Events & extract Aggregates
		aggregates, errs, err := job.Aggregates(job.Context())
		// aggregates == []event.AggregateTuple

		// Fetch Events & extract Aggregates with given names
		aggregates, errs, err := job.Aggregates(job.Context(), "foo", "bar", "baz")
		// aggregates == []event.AggregateTuple

		// Fetch Events & extract UUID of first Aggregate with given name
		id, err := job.Aggregate(job.Context(), "foo")

		// Apply Job onto given Target
		err := job.Apply(job.Context(), proj)
	})
}
```

### Trigger schedule

```go
package example

func triggerSchedule(ctx context.Context, s projection.Schedule) error {
	return s.Trigger(ctx)
}
```

### Trigger schedule with additional filter

When provided a `projection.Filter`, the created Job of the triggered schedule
modifies the query used to fetch the needed events from the event store.

```go
package example

func triggerSchedule(ctx context.Context, s projection.Schedule) error {	
	return s.Trigger(ctx, projection.Filter(query.New(
		query.AggregateName("foo"),
		// ...
	)))
}
```

## Projection guards

A `Guard` protects a projection from unwanted/unexpected Events.

```go
package example

type exampleProjection struct {
	projection.Guard
}

func newExampleProjection() *exampleProjection {
	return &exampleProjection{
		Guard: projection.Guard(query.New(
			query.AggregateName("foo", "bar", "baz"),
			// ...
		)),
	}
}

func (*exampleProjection) ApplyEvent(event.Event) {}

// not needed to be implemented, just here to show how it works
func (ex *exampleProjection) GuardProjection(evt event.Event) bool {
	return query.Test(ex.Guard, evt)
}

func subscribeToSchedule(ctx context.Context, s projection.Schedule) (<-chan error, error) {
	return s.Subscribe(ctx, func(job projection.Job) error {
		proj := newExampleProjection()

		// Apply Job onto projection. proj.GuardProjection is used to determine if
		// an Event should be applied onto the projection.
		err := job.Apply(job.Context(), proj)
	})
}
```

## Projection service

The projection `Service` allows intra-service triggering of Schedules.
Communication between services is done over an event bus.

### Create service

```go
package example

func newProjectionService(reg event.Registry, bus event.Bus) *projection.Service {
	return projection.NewService(reg, bus)
}
```

### Create service and register schedules

```go
package example

func newProjectionService(reg event.Registry, bus event.Bus, store event.Store) *projection.Service {
	schedule := schedule.Continuously(bus, store, []string{"foo", "bar", "baz"})
	return projection.NewService(reg, bus, projection.RegisterSchedule("example", schedule))
}
```

### Run service

```go
package example

func runProjectionService(ctx context.Context, svc *projection.Service) (<-chan error, error) {
	return svc.Run(ctx)
}
```

### Trigger schedule

```go
package example

func triggerSchedule(ctx context.Context, svc *projection.Service, name string) error {
	return svc.Trigger(ctx, name)
}
```

### Trigger schedule with additional filter

```go
package example

func triggerSchedule(ctx context.Context, svc *projection.Service, name string) error {	
	return svc.Trigger(ctx, name, projection.Filter(query.New(
		query.AggregateName("foo"),
		// ...
	)))
}
```
