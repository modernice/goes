# goes - Event-Sourcing Framework for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/modernice/goes.svg)](https://pkg.go.dev/github.com/modernice/goes)
[![Documentation](https://img.shields.io/badge/Docs-goes.modernice.dev-blue)](https://goes.modernice.dev)

<p align="center">
  <img src="./docs/assets/goes_logo.png" alt="goes gopher logo" width="320">
</p>

`goes` is an event-sourcing framework for Go. It gives you the building blocks to model domain logic with aggregates, persist state as events, build read models with projections, and wire the same application to in-memory or production backends.

## Why goes?

- Typed aggregates, events, commands, and repositories with less boilerplate
- Backend-agnostic application code that works with in-memory, MongoDB, PostgreSQL, and NATS backends
- Streaming-first query and subscription APIs that return channels, keeping large workloads incremental and low-memory
- Production-oriented features like optimistic concurrency, snapshots, and continuous projections
- Durable, event-driven workflows (sagas) that coordinate long-running processes with timeouts and compensation
- A practical path from local development to distributed systems without changing your core domain model

## Install

`goes` currently targets Go 1.25+.

```bash
go get github.com/modernice/goes/...
```

The `/...` suffix downloads the framework packages together with the backend implementations.

## Quick Start

This is the smallest useful path: define an aggregate, save it to an in-memory event store, then fetch it back by replaying events.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
)

const (
	listAggregate = "todo.list"
	listCreated   = "todo.list.created"
	itemAdded     = "todo.list.item_added"
)

type (
	listCreatedEvent = event.Of[string]
	itemAddedEvent   = event.Of[string]
)

type List struct {
	*aggregate.Base
	Title string
	Items []string
}

func NewList(id uuid.UUID) *List {
	l := &List{Base: aggregate.New(listAggregate, id)}
	event.ApplyWith(l, l.created, listCreated)
	event.ApplyWith(l, l.added, itemAdded)
	return l
}

func (l *List) Create(title string) {
	aggregate.Next(l, listCreated, title)
}

func (l *List) AddItem(item string) {
	aggregate.Next(l, itemAdded, item)
}

func (l *List) created(evt listCreatedEvent) {
	l.Title = evt.Data()
}

func (l *List) added(evt itemAddedEvent) {
	l.Items = append(l.Items, evt.Data())
}

func main() {
	ctx := context.Background()
	store := eventstore.New()
	lists := repository.Typed(repository.New(store), NewList)

	id := uuid.New()
	list := NewList(id)
	list.Create("Groceries")
	list.AddItem("Milk")
	list.AddItem("Eggs")

	if err := lists.Save(ctx, list); err != nil {
		log.Fatal(err)
	}

	fetched, err := lists.Fetch(ctx, id)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(fetched.Title, fetched.Items)
}
```

What happens here:

- `List` is an aggregate that raises events instead of mutating persisted state directly
- `aggregate.Next` records and applies events immediately
- `repository.Typed(...)` saves uncommitted events and reconstructs aggregates on fetch
- `eventstore.New()` keeps everything in memory, so you can prototype and test without infrastructure

## Core Concepts

- `aggregate` - define consistency boundaries that own state and business rules
- `event` - describe immutable facts and store or publish them
- `aggregate/repository` - save and rehydrate aggregates from an event store
- `projection` - build read-optimized views from event streams
- `command` - coordinate intent across aggregates and services when needed
- `codec` - register event and command types for serialization across backends

## Backends

Application code stays on framework interfaces like `event.Store` and `event.Bus`. Pick the backend at startup.

| Component | In-memory | Production options |
| --- | --- | --- |
| Event store | `event/eventstore` | `backend/mongo`, `backend/postgres` |
| Event bus | `event/eventbus` | `backend/nats` |
| Snapshots | - | `backend/mongo` |
| Read models | `backend/memory` | `backend/mongo` |

Use the in-memory backends for tests and local experiments. Use MongoDB or PostgreSQL for persisted event streams and NATS for distributed event delivery.

## Event Store UI

The repository includes a standalone web console for MongoDB and PostgreSQL event stores. It supports multiple named connections, event and aggregate filters, stream inspection, server-side payload decoding, and optional static login protection. A single Docker image serves both the Nuxt frontend and Go API.

```bash
docker pull ghcr.io/modernice/goes-ui:latest
```

Configuration is supplied through `GOES_UI_*` environment variables or Docker secrets. The stock image handles JSON event payloads; applications with custom codecs can inject their existing `codec.Encoding` through the public `eventstoreui` package. See the [Event Store UI guide](https://goes.modernice.dev/tooling/event-store-ui) for configuration and Docker Swarm examples.

## Testing

Event-sourced aggregates are easy to test because they are in-memory state machines. `goes` also ships `github.com/modernice/goes/exp/gtest` for aggregate-focused assertions and provides in-memory backends for integration tests without external services.

- Guide: [Testing](https://goes.modernice.dev/guide/testing)
- Package docs: [`exp/gtest`](https://pkg.go.dev/github.com/modernice/goes/exp/gtest)

## Learn More

- Documentation site: [goes.modernice.dev](https://goes.modernice.dev)
- Getting started: [Introduction](https://goes.modernice.dev/getting-started/introduction)
- Tutorial: [Build an event-sourced app](https://goes.modernice.dev/tutorial/)
- Backends: [Overview](https://goes.modernice.dev/backends/)
- Workflows (sagas): [Guide](https://goes.modernice.dev/guide/workflows) and the [`workflow`](./workflow) package
- Reference: [Architecture](https://goes.modernice.dev/reference/architecture) and [Best Practices](https://goes.modernice.dev/reference/best-practices)
- Distributed example app: [`examples/todo/`](./examples/todo)

## Community

- Questions and ideas: [GitHub Discussions](https://github.com/modernice/goes/discussions)
- Bugs and feature requests: [GitHub Issues](https://github.com/modernice/goes/issues/new)

## License

[Apache License, Version 2.0](./LICENSE)
