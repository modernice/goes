# Projection

`projection` helps build read models by replaying events. Any value that
implements `Target` can consume events to update its state.

```go
 type Target interface {
     ApplyEvent(event.Event)
 }
```

Use `Apply` to feed events into a target. Targets may implement `Guard` to
reject events and `ProgressAware` to skip ones that were already processed.

```go
projection.Apply(target, events)
```

## Schedules

Schedules produce projection jobs from an event source.

- `schedule.Continuous` listens on a bus and emits jobs as events arrive. The
  `Debounce` option batches bursts of events.
- `schedule.Periodic` polls the store on a fixed interval.

A job exposes helpers to query and apply the events it wraps.

## Services

`Service` coordinates schedules over an event bus. Triggers can reset
projections, override queries or add filters before jobs are built.

See subpackages `guard`, `lookup` and `schedule` for specialised helpers.
