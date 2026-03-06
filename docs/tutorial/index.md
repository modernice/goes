# Tutorial: Build an E-Commerce App

In this tutorial, you'll build an event-sourced e-commerce application from scratch. By the end, you'll have a working system with products, orders, customers, and reactive read models.

## What You'll Build

- **Product** aggregate — create products, manage prices and stock
- **Order** aggregate — place orders with line items, cancel orders
- **Customer** aggregate — manage customer profiles and addresses
- **Product Catalog** projection — a read model that stays in sync with product events

## What You'll Learn

| Chapter | Concept |
| --- | --- |
| [1. Project Setup](./01-project-setup) | Go module, in-memory backends, bootstrap |
| [2. Your First Aggregate](./02-first-aggregate) | `aggregate.Base`, events, appliers |
| [3. Events & State](./03-events-and-state) | Multiple events, typed event data |
| [4. Codec Registry](./04-codec-registry) | Event serialization |
| [5. Repositories](./05-repository) | Saving and fetching aggregates |
| [6. Commands](./06-commands) | Command bus, dispatching, handlers |
| [7. The Order](./07-order-aggregate) | Second aggregate, validation patterns |
| [8. The Customer](./08-customer-aggregate) | Value objects, addresses |
| [9. Projections](./09-projections) | Read models, schedules |
| [10. Production Backends](./10-backends) | MongoDB, NATS, PostgreSQL |
| [11. Testing](./11-testing) | Testing aggregates and projections |

## Prerequisites

- Basic Go knowledge (structs, interfaces, generics)
- Familiarity with event sourcing and DDD concepts (see [Introduction](/getting-started/introduction))

## The Colocation Pattern

Throughout this tutorial, we keep everything related to an aggregate in a single file. Event names, event data types, commands, the aggregate struct, business methods, and event appliers all live together. This makes aggregates self-contained and easy to understand.

```
shop/
  main.go           # Bootstrap, wiring
  product.go        # Product aggregate + events + commands
  order.go          # Order aggregate + events + commands
  customer.go       # Customer aggregate + events + commands
  catalog.go        # Product catalog projection
```

Let's get started.
