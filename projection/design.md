# Projections

## Schedules

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