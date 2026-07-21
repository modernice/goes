# Example – Workflows

Runnable examples for the [`workflow`](../../workflow) package. Every example
is a self-contained program using in-memory backends — no external services
required:

```sh
go run ./basic
```

## Scenarios

### [`basic`](./basic)

The fundamental lifecycle: a trigger event starts a workflow
(`workflow.Starts`), the workflow records a command effect (`Ctx.Dispatch`)
and schedules a payment deadline (`Ctx.Schedule`), a second trigger
(`workflow.Reacts`) cancels the deadline and completes the workflow. Uses the
built-in `workflow.ByAggregateID` correlator and a real command bus with a
command handler.

### [`correlators`](./correlators)

The different ways trigger events find their workflow instance:

- `workflow.ByKey`: deriving the workflow id from a business key in the
  event payload (one loyalty workflow per customer email), collecting events
  from many different aggregates into a single workflow
- hand-written correlators with selective correlation: returning `false` to
  ignore events entirely (orders below a minimum amount)
- one handler registered for multiple event names sharing a payload type
- `workflow.Starts` acting as an upsert: the first qualified order creates
  the workflow, further orders advance it through the same handler
- `Config.Strict`: events correlating to unknown workflows become errors
  instead of being silently ignored

### [`timeouts`](./timeouts)

Everything around timeouts: multiple concurrent timeout keys per workflow (a
reminder and a deadline), a timeout handler that leaves the workflow running,
extending a deadline by rescheduling its key, canceling timeouts, and a
deadline that fails the workflow (`Ctx.Fail`).

### [`compensation`](./compensation)

The compensation phase: a failed payment triggers `Ctx.Compensate`, the
workflow dispatches a compensating command and guards it with a compensation
deadline (`workflow.OnCompensationTimeout`). One order compensates
successfully (`Ctx.Compensated` → `StatusCompensated`), another loses its
stock release and fails (`Ctx.CompensationFailed` → `StatusFailed`).
Demonstrates phase-based routing: `workflow.Compensates` handlers only run
while the workflow is compensating.

### [`state`](./state)

Workflows with their own event-sourced state: a document review workflow
records every approval as a custom workflow event (`aggregate.Next` +
`event.ApplyWith`), deduplicates reviewers at the business level, and
completes once two distinct reviewers approved.

### [`recovery`](./recovery)

Durability across restarts: a trigger persisted while no service was running
is picked up by the startup trigger replay; an instance crashes before its
scheduled timeout fires; a fresh instance recovers the overdue timeout from
the event store, fires it, and does not re-dispatch the command the first
instance already dispatched.

### [`cluster`](./cluster)

Multiple service instances sharing one event store: effects recorded by one
instance are discovered and executed by another through periodic resyncs
(`Config.ResyncInterval`), and a stale instance never fires a timeout that
another instance already canceled — effect execution is always validated
against the event store.
