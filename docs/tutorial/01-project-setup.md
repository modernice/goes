# 1. Project Setup

Let's set up the Go project and wire together the foundation of our e-commerce application.

## Create the Module

```bash
mkdir shop && cd shop
go mod init github.com/yourname/shop
go get github.com/modernice/goes/...
```

## Project Structure

Domain code lives in the root package (`package shop`). The binary lives in `cmd/`:

```
shop/
  product.go     # Product aggregate (coming in chapter 2)
  order.go       # Order aggregate (coming in chapter 7)
  customer.go    # Customer aggregate (coming in chapter 8)
  catalog.go     # Product catalog projection (coming in chapter 10)
  cmd/
    main.go      # Bootstrap and wiring
```

## Bootstrap

Create `cmd/main.go` with the foundation. We'll use in-memory backends to start — no databases needed.

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
)

func main() {
	// Graceful shutdown on Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Codec registries — one for events, one for commands.
	eventReg := codec.New()
	cmdReg := codec.New()

	// In-memory event store and event bus.
	store := eventstore.New()
	bus := eventbus.New()

	// Wire the event store to publish events on the bus when they are inserted.
	store = eventstore.WithBus(store, bus)

	// We'll register events, set up repositories, and handle commands here.
	_, _, _, _ = eventReg, cmdReg, store, ctx

	log.Println("Shop is running. Press Ctrl+C to stop.")
	<-ctx.Done()
}
```

Run it with:

```bash
go run ./cmd
```

### What's Happening?

- **`codec.New()`** creates a registry that maps names to Go types for serialization. We create two — `eventReg` for event data and `cmdReg` for command payloads. Keeping them separate avoids name collisions and makes the wiring explicit.
- **`eventstore.New()`** creates an in-memory event store. Events are stored in memory — fine for development.
- **`eventbus.New()`** creates an in-memory event bus for pub/sub.
- **`eventstore.WithBus(store, bus)`** wraps the store so that events are automatically published to the bus whenever they're inserted. This is what makes projections reactive.

In [chapter 11](./11-backends), we'll swap these in-memory implementations for MongoDB and NATS.

## Next

In the [next chapter](./02-first-aggregate), we'll create our first aggregate — the Product.
