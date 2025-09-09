# Aggregates

Package `aggregate` provides building blocks for event-sourced domain models.
It sits on top of the [`event`](../event) package and exposes utilities for
recording and replaying changes, repositories, queries and snapshots.

## Getting started

Embed `*aggregate.Base` into your type or implement the [`Aggregate`](./api.go)
interface yourself:

```go
type TodoList struct{ *aggregate.Base }

func NewList(id uuid.UUID) *TodoList {
    return &TodoList{Base: aggregate.New("list", id)}
}
```

Call [`aggregate.Next`](./base.go) to create and apply events. `Base` collects
uncommitted events and satisfies [`Committer`](./api.go),
[`repository.ChangeDiscarder`](./repository/retry.go) and
[`snapshot.Aggregate`](./snapshot).

## Soft deletion

Event data may implement [`SoftDeleter`](./api.go) or
[`SoftRestorer`](./api.go) to mark aggregates as deleted or restored. The
[`Repository`](./repository) ignores soft-deleted aggregates and returns
[`repository.ErrDeleted`](./repository/repository.go) when fetching them.

## Repositories and queries

[`Repository`](./repository.go) persists aggregates by storing their events and
optionally using snapshots. The [`repository`](./repository) subpackage adds
helpers such as typed repositories and in-memory caching. The
[`query`](./query) package builds rich aggregate filters that translate to event
queries.

## Snapshots

Aggregates can implement `snapshot.Target` to enable snapshotting. The
[`snapshot`](./snapshot) package defines stores and schedules used by the
repository to load and persist snapshots.
