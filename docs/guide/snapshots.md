# Snapshots

Every time you fetch an aggregate, the repository replays all its events from the beginning. For aggregates with many events, this gets slow. Snapshots capture the aggregate's state at a point in time, so only events after the snapshot need replaying.

## How It Works

1. **On Save** — after inserting events, the repository checks the snapshot schedule. If the schedule says yes, it serializes the aggregate and saves a snapshot.
2. **On Fetch** — the repository loads the latest snapshot, sets the aggregate's version and state, then replays only events with a version higher than the snapshot.

This turns a replay of 10,000 events into loading one snapshot + replaying the last few events.

## Snapshot Store

The snapshot store persists snapshots. All implementations satisfy the `snapshot.Store` interface:

```go
import "github.com/modernice/goes/aggregate/snapshot"
```

| Method | Description |
| --- | --- |
| `Save(ctx, snapshot)` | Save a snapshot |
| `Latest(ctx, name, id)` | Get the most recent snapshot for an aggregate |
| `Version(ctx, name, id, version)` | Get a snapshot at a specific version |
| `Limit(ctx, name, id, version)` | Get the latest snapshot at or below a version |
| `Query(ctx, query)` | Stream snapshots matching a query |
| `Delete(ctx, snapshot)` | Delete a snapshot |

`Latest` is the most common — the repository calls it on every `Fetch` to skip replaying events the snapshot already covers.

### Implementations

- **MongoDB** — `mongo.NewSnapshotStore(opts...)` — see [MongoDB backend](/backends/mongodb#snapshot-store)
- **[In-memory](/backends/in-memory)** — useful for testing

## Snapshot Schedule

The schedule determines when to take snapshots. The built-in option:

```go
schedule := snapshot.Every(100)
```

This takes a snapshot every 100 events. More precisely, it returns `true` if any version in the range `(lastSnapshotVersion, currentVersion]` is divisible by 100.

For custom logic, implement the `snapshot.Schedule` interface:

```go
type Schedule interface {
	Test(aggregate.Aggregate) bool
}
```

## Configuring the Repository

Wire the snapshot store and schedule into the repository:

```go
import (
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/aggregate/snapshot"
	gomongo "github.com/modernice/goes/backend/mongo"
)

snapshots := gomongo.NewSnapshotStore(
	gomongo.SnapshotURL("mongodb://localhost:27017"),
)

repo := repository.New(store,
	repository.WithSnapshots(snapshots, snapshot.Every(100)),
)
```

That's all the wiring needed. The repository handles saving and loading snapshots automatically.

## Encoding

For the repository to serialize your aggregate into a snapshot, the aggregate must implement one of these interfaces (checked in order):

1. `snapshot.Marshaler` / `snapshot.Unmarshaler`
2. `encoding.BinaryMarshaler` / `encoding.BinaryUnmarshaler`
3. `encoding.TextMarshaler` / `encoding.TextUnmarshaler`

The simplest approach — marshal the DTO to JSON:

```go
import "encoding/json"

func (p *Product) MarshalSnapshot() ([]byte, error) {
	return json.Marshal(p.ProductDTO)
}

func (p *Product) UnmarshalSnapshot(b []byte) error {
	return json.Unmarshal(b, &p.ProductDTO)
}
```

::: tip
Only serialize the DTO, not the entire aggregate. `*aggregate.Base` handles its own version tracking — the snapshot system restores the version separately.
:::

## When to Use Snapshots

Snapshots add complexity. Use them when:

- Aggregates commonly accumulate 100+ events
- Fetch latency is noticeable in your application
- You've measured that event replay is the bottleneck

For most applications, aggregates have a manageable number of events and snapshots aren't needed initially. Add them later when performance data justifies it.
