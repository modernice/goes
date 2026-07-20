# Cookbook

These recipes cover application patterns that are common in larger goes systems but intentionally sit a level above the beginner guides.

They are not the only way to build with goes. They are the patterns that become useful once your application has more services, more read models, and more operational concerns.

## Dirty-set then finalize

### Use it when

Use this pattern when event payloads are enough to identify what changed, but not enough to fully rebuild the read model.

Typical examples:

- a projection can mark which aggregates became dirty from events alone
- the final view needs a repository fetch or another expensive read
- you want to batch multiple rapid events into one rebuild pass

### Pattern

1. Apply events cheaply and only record dirty IDs.
2. At the end of the job, deduplicate the dirty IDs.
3. Re-fetch the current aggregate state once per dirty ID.
4. Rebuild any secondary indexes or derived caches after the fetch step.

```go
type ContactList struct {
	*projection.Base

	mux      sync.RWMutex
	contacts map[uuid.UUID]ContactView
	dirty    []uuid.UUID
}

func NewContactList() *ContactList {
	l := &ContactList{
		Base:     projection.New(),
		contacts: make(map[uuid.UUID]ContactView),
		dirty:    make([]uuid.UUID, 0),
	}

	event.ApplyWith(l, l.contactUpdated, ContactUpdated)
	event.ApplyWith(l, l.photoUploaded, PhotoUploaded)

	return l
}

func (l *ContactList) contactUpdated(evt event.Of[ContactUpdatedData]) {
	id := pick.AggregateID(evt)
	view := l.contacts[id]
	view.ID = id
	view.Name = evt.Data().Name
	view.Email = evt.Data().Email
	l.contacts[id] = view
}

func (l *ContactList) photoUploaded(evt event.Event) {
	l.dirty = append(l.dirty, pick.AggregateID(evt))
}

func (l *ContactList) Run(ctx context.Context, bus event.Bus, store event.Store, contacts ContactRepository) (<-chan error, error) {
	s := schedule.Continuously(bus, store, l.RegisteredEvents(), schedule.Debounce(250*time.Millisecond))

	return s.Subscribe(ctx, func(job projection.Job) error {
		l.mux.Lock()
		defer l.mux.Unlock()

		if err := job.Apply(job, l); err != nil {
			return err
		}

		for _, id := range unique(l.dirty) {
			contact, err := contacts.Fetch(job, id)
			if err != nil {
				return fmt.Errorf("fetch contact: %w", err)
			}

			l.contacts[id] = ContactView{
				ID:    contact.ID,
				Name:  contact.Name,
				Email: contact.Email,
				Photo: contact.PhotoURL,
			}
		}

		l.dirty = l.dirty[:0]
		return nil
	}, projection.Startup())
}
```

### Why this works

- cheap events stay cheap
- fetches happen once per aggregate per job, not once per event
- projections can still keep their hot path simple

### Tradeoff

This is more complex than plain `job.Apply(...)`. Do not reach for it if the read model can already be updated directly from event data.

## Operational snapshot generation

The [Snapshots](/guide/snapshots) guide covers automatic repository snapshots. Larger systems often also need explicit snapshot jobs:

- backfill snapshots after introducing snapshot support
- prewarm snapshots for high-traffic aggregates
- snapshot only selected aggregate families

### Pattern

Create an application service that queries the target aggregates, builds snapshots explicitly, and saves them to the snapshot store.

```go
type SnapshotService struct {
	store    snapshot.Store
	profiles ProfileRepository
}

func NewSnapshotService(store snapshot.Store, profiles ProfileRepository) *SnapshotService {
	return &SnapshotService{
		store:    store,
		profiles: profiles,
	}
}

func (svc *SnapshotService) GenerateProfiles(ctx context.Context) error {
	stream, errs, err := svc.profiles.Query(ctx, query.New())
	if err != nil {
		return fmt.Errorf("query profiles: %w", err)
	}

	return streams.Walk(ctx, func(profile *Profile) error {
		if profile.CurrentVersion() == 0 {
			return nil
		}

		snap, err := snapshot.New(profile)
		if err != nil {
			return fmt.Errorf("create snapshot: %w", err)
		}

		if err := svc.store.Save(ctx, snap); err != nil {
			return fmt.Errorf("save snapshot: %w", err)
		}

		return nil
	}, stream, errs)
}
```

### Selective snapshot-backed repositories

You do not need one repository configuration for the entire service. A practical pattern is to keep both:

```go
repo := repository.New(store)

snapshotRepo := repository.New(
	store,
	repository.WithSnapshots(snapshots, snapshot.Every(100)),
)

properties := repository.Typed(repo, NewProperty)
profiles := repository.Typed(snapshotRepo, NewProfile)
galleries := repository.Typed(snapshotRepo, GalleryOf)
```

Use snapshot-backed repositories for the aggregates with long histories or hot read paths. Keep the rest on the simpler base repository.

### When to prefer an explicit snapshot job

- you are introducing snapshots to an existing dataset
- you need to precompute snapshots before a rollout
- you want operational control over when the expensive work happens

## Bounded migrations and backfills

Migrations get safer when they are explicit about which aggregates they read and how they replay work into the new system.

### Pattern

1. Query a bounded slice of aggregates.
2. Feed IDs into a worker queue.
3. Rehydrate aggregate state from the old stream.
4. Dispatch a synchronous command into the new write model.

```go
func MigrateOrders(ctx context.Context, oldOrders aggregate.TypedRepository[*LegacyOrder], bus command.Bus) error {
	stream, errs, err := oldOrders.Query(ctx, query.New(
		query.Version(version.InRange(version.Range{0, 3})),
	))
	if err != nil {
		return fmt.Errorf("query legacy orders: %w", err)
	}

	queue := make(chan uuid.UUID)
	queueErr := make(chan error, 1)

	go func() {
		defer close(queue)
		queueErr <- streams.Walk(ctx, func(order *LegacyOrder) error {
			queue <- order.ID
			return nil
		}, stream, errs)
	}()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range queue {
				order, err := oldOrders.Fetch(ctx, id)
				if err != nil {
					continue
				}

				cmd := ImportOrder(order.AggregateID(), ImportOrderParams{
					Number: order.Number,
					Total:  order.Total,
				})

				if err := bus.Dispatch(ctx, cmd.Any(), dispatch.Sync()); err != nil {
					// record and continue, or collect failures
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-queueErr:
		return err
	case <-done:
		return nil
	}
}
```

### Why these pieces matter

- `aggregate/query` and `event/query/version` let you bound the migration set instead of replaying the whole store blindly
- `streams.Walk(...)` drains both result and error channels correctly
- worker fan-out keeps the migration throughput predictable
- `dispatch.Sync()` makes command failures visible immediately instead of hiding them behind asynchronous processing

### Rules of thumb

- make migrations idempotent if possible
- bound the input set explicitly
- prefer dispatching commands into the new model instead of mutating new aggregates out of band
- log or collect per-item failures instead of aborting the whole run on the first bad record

## Related guides

- [Commands](/guide/commands)
- [Projections](/guide/projections)
- [Lookups](/guide/lookups)
- [Snapshots](/guide/snapshots)
