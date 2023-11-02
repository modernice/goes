<script lang="ts" setup>
import CardLinks from '../../../components/CardLinks.vue'
import CardLink from '../../../components/CardLink.vue'
</script>
# Events

goes defines and implements a generic event system that is used as the building
block for [aggregates](/guide/aggregates/), [commands](/guide/commands/), and
[projections](/guide/projections/).

## Event System

The event system consists of 3 interfaces – `event.Event / event.Of[T]`,
`event.Bus`, and `event.Store`. All other components of goes are built on top of
this system.

[View type definitions](#type-definitions)

## Kinds of Events

An event can be either be a "normal" event, or an **Aggregate Event**, differing
only in the data they provide. Normal events provide a unique id, the event name,
event time, and some arbitrary [Event Data](/guide/events/creating-events#event-data).
Aggregate Events additionally provide the position of the event within the event
stream of an aggregate.

## Backends

goes provides backend implementations for the `event.Bus` and `event.Store` interfaces:

### Event Bus

<CardLinks>
<CardLink
  title="In-Memory"
  description="In-memory event bus for local development. Does not support inter-service communication."
  link="/guide/backends/event-bus/in-memory"
/>
<CardLink
  title="NATS Core / JetStream"
  description="Production-ready event bus with support for inter-service communication."
  link="/guide/backends/event-bus/nats"
/>
</CardLinks>

### Event Store

<CardLinks>
<CardLink
  title="In-Memory"
  description="Non-persistent, in-memory event store for local development."
  link="/guide/backends/event-store/in-memory"
/>
<CardLink
  title="MongoDB"
  description="Production-ready event store, powered by MongoDB."
  link="/guide/backends/event-store/mongodb"
/>
<CardLink
  title="Postgres (Beta)"
  description="An event store, powered by Postgres."
  link="/guide/backends/event-store/postgres"
/>
</CardLinks>

## Type Definitions

### Event

<<< @/../../event/event.go#event

### Bus

<<< @/../../event/bus.go#bus

### Store

<<< @/../../event/store.go#store
