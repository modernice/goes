# 11. Production Backends

So far we've used in-memory event store and event bus. They're great for development, but events are lost when the process stops. Let's swap in production backends.

## Available Backends

| Component | Options |
| --- | --- |
| Event Store | [MongoDB](/backends/mongodb), [PostgreSQL](/backends/postgres), [In-Memory](/backends/in-memory) |
| Event Bus | [NATS](/backends/nats), [In-Memory](/backends/in-memory) |

A typical production setup uses **[MongoDB](/backends/mongodb)** (or [PostgreSQL](/backends/postgres)) for the event store and **[NATS](/backends/nats)** for the event bus.

## MongoDB Event Store

Install the MongoDB driver (it's already included in goes):

```go
import "github.com/modernice/goes/backend/mongo"
```

Replace the [in-memory event store](/backends/in-memory):

```go
// Before (in-memory):
store := eventstore.New()

// After (MongoDB):
mongoStore := mongo.NewEventStore(eventReg,
	mongo.URL("mongodb://localhost:27017"),
	mongo.Database("shop"),
)
```

The MongoDB event store handles:
- Event persistence with optimistic concurrency
- Indexed queries by aggregate, event name, and time
- Automatic collection setup

## PostgreSQL Event Store

Alternatively, use PostgreSQL:

```go
import "github.com/modernice/goes/backend/postgres"
```

```go
pgStore := postgres.NewEventStore(eventReg,
	postgres.URL("postgres://localhost:5432/shop?sslmode=disable"),
)
```

## NATS Event Bus

For a distributed event bus, use NATS:

```go
import "github.com/modernice/goes/backend/nats"
```

```go
natsBus := nats.NewEventBus(eventReg,
	nats.URL("nats://localhost:4222"),
)
```

NATS supports two modes:
- **Core** — simple pub/sub, at-most-once delivery
- **JetStream** — persistent streams, at-least-once delivery, replay

```go
// Use JetStream for persistence:
natsBus := nats.NewEventBus(eventReg,
	nats.URL("nats://localhost:4222"),
	nats.Use(nats.JetStream()),
)
```

## Updated main.go

Here's how `cmd/main.go` changes for production:

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/backend/memory"
	gomongo "github.com/modernice/goes/backend/mongo"
	gonats "github.com/modernice/goes/backend/nats"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/yourname/shop"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	eventReg := codec.New()
	shop.RegisterProductEvents(eventReg)
	shop.RegisterOrderEvents(eventReg)
	shop.RegisterCustomerEvents(eventReg)

	cmdReg := codec.New()
	shop.RegisterProductCommands(cmdReg)
	shop.RegisterOrderCommands(cmdReg)
	shop.RegisterCustomerCommands(cmdReg)

	// Production event bus (NATS).
	bus := gonats.NewEventBus(eventReg,
		gonats.URL("nats://localhost:4222"),
	)

	// Production event store (MongoDB), wired to publish on bus.
	store := eventstore.WithBus(gomongo.NewEventStore(eventReg,
		gomongo.URL("mongodb://localhost:27017"),
		gomongo.Database("shop"),
	), bus)

	repo := repository.New(store)
	products := repository.Typed(repo, shop.NewProduct)
	orders := repository.Typed(repo, shop.NewOrder)
	customers := repository.Typed(repo, shop.NewCustomer)

	cbus := cmdbus.New[int](cmdReg, bus)

	productErrs := shop.HandleProductCommands(ctx, cbus, products)
	orderErrs := shop.HandleOrderCommands(ctx, cbus, orders, products)
	customerErrs := shop.HandleCustomerCommands(ctx, cbus, customers)

	catalog := shop.NewProductCatalog()
	catalogErrs, err := catalog.Run(ctx, bus, store)
	if err != nil {
		log.Fatal(err)
	}

	shopStatsRepo := memory.NewModelRepository[*shop.ShopStats, uuid.UUID](
		memory.ModelFactory(shop.NewShopStats),
	)
	statsErrs, err := shop.RunShopStats(ctx, bus, store, shopStatsRepo)
	if err != nil {
		log.Fatal(err)
	}

	orderSummaries := memory.NewModelRepository[*shop.OrderSummary, uuid.UUID](
		memory.ModelFactory(shop.OrderSummaryOf),
	)
	summaryProjector := shop.NewOrderSummaryProjector(customers, orders, orderSummaries)
	summaryErrs, err := summaryProjector.Run(ctx, bus, store)
	if err != nil {
		log.Fatal(err)
	}

	orderHistories := memory.NewModelRepository[*shop.CustomerOrderHistory, uuid.UUID](
		memory.ModelFactory(shop.OrderHistoryOf),
	)
	historyProjector := shop.NewCustomerOrderHistoryProjector(customers, orders, orderHistories)
	historyErrs, err := historyProjector.Run(ctx, bus, store)
	if err != nil {
		log.Fatal(err)
	}

	go logErrors(productErrs)
	go logErrors(orderErrs)
	go logErrors(customerErrs)
	go logErrors(catalogErrs)
	go logErrors(statsErrs)
	go logErrors(summaryErrs)
	go logErrors(historyErrs)

	log.Println("Shop is running. Press Ctrl+C to stop.")
	<-ctx.Done()
}

func logErrors(errs <-chan error) {
	for err := range errs {
		log.Printf("Error: %v", err)
	}
}
```

## Docker Compose for Local Development

Create a `docker-compose.yml` to run MongoDB and NATS locally:

```yaml
services:
  mongodb:
    image: mongo:8
    ports:
      - "27017:27017"
    volumes:
      - mongo-data:/data/db

  nats:
    image: nats:2
    ports:
      - "4222:4222"
      - "8222:8222"  # monitoring
    command: ["--js"]  # enable JetStream

volumes:
  mongo-data:
```

```bash
docker compose up -d
go run ./cmd
```

## `eventstore.WithBus`

The `eventstore.WithBus(store, bus)` decorator is important — it wraps the event store so that whenever events are inserted, they're also published to the event bus. This is what makes projections reactive.

Without this wrapper, you'd need to manually publish events after saving them.

## Next

Everything works end-to-end. In the [final chapter](./12-testing), we'll write tests for our aggregates and projections.
