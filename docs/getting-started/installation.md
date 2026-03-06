# Installation

## Prerequisites

- **Go 1.21** or later
- A Go module (run `go mod init` if you don't have one)

## Install

```bash
go get github.com/modernice/goes/...
```

The `/...` suffix ensures all nested packages are downloaded, including backend implementations.

## Module Structure

goes is organized into focused packages:

| Package | What it does |
| --- | --- |
| `aggregate` | Define domain objects that own state and enforce business rules |
| `aggregate/repository` | Save and load aggregates from storage |
| `aggregate/snapshot` | Speed up aggregate loading by caching their state at a point in time |
| `event` | Define events, publish and subscribe to them, store and query them |
| `command` | Define commands and route them to the right handler |
| `command/cmdbus` | Command bus that dispatches commands over the event system |
| `projection` | Build read-optimized views from events |
| `codec` | Register event and command types so they can be serialized |

### Backend Packages

| Package | What it does |
| --- | --- |
| `backend/mongo` | Store events, snapshots, and read models in MongoDB |
| `backend/postgres` | Store events in PostgreSQL |
| `backend/nats` | Publish and subscribe to events over NATS |

### In-Memory (Testing & Prototyping)

| Package | What it does |
| --- | --- |
| `event/eventstore` | Store events in memory (no database needed) |
| `event/eventbus` | Publish and subscribe to events in memory (no message broker needed) |

## What's Next?

- [Quick Start](/getting-started/quick-start) — a minimal working example
- [Tutorial](/tutorial/) — build a full e-commerce app
