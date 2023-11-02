# What is goes?

**goes** is an open-source framework designed to implement event-sourcing in
applications written in Go. It provides a robust set of tools and components
that make it easier to build applications following the principles of
event-driven architecture.

## Simplifying Event-Sourcing

Event-sourcing is an architectural pattern that revolves around capturing all
changes to an application's state as a sequence of events. This pattern ensures
that all state transitions are explicit, reliable, and replayable, which can be
invaluable for debugging, auditing, and building scalable systems.

goes simplifies the implementation of event-sourcing by handling the complex
plumbing often associated with event-driven systems. It provides a comprehensive
ecosystem for managing events, commands, and aggregates, allowing developers to
focus on the business logic rather than the intricacies of the underlying architecture.

## Core Features

The framework comes with a variety of features that are essential for event-driven development:

### Event System

goes features an advanced [Event System](/core/events/) that encompasses both an
event bus and an event store, providing a comprehensive solution for event
management. This system allows applications to publish, subscribe to, and
persist events efficiently, facilitating communication between different
parts of the system without tight coupling.

### Aggregate Framework

With the aggregate framework, you can build and manage business entities with
encapsulated logic and state, using events to transition through states effectively.

### Command System

The command system is designed for distributed services that need to dispatch
commands to each other. It uses the event system to facilitate the communication
between services, ensuring that commands can be handled across a distributed system.

### Projection Toolkit

The projection toolkit enables the transformation of event streams into readable
and queryable views, ensuring data can be consumed in the most effective way by
different parts of the application.

### Modular Design

goes is built with modularity in mind. You can pick and choose which components
to use and when to use them, allowing for incremental adoption and flexibility.

### Efficiency

Performance is a key focus of goes. It is designed to handle event streams with
minimal memory overhead, ensuring your applications run efficiently, even under high loads.

## Current Constraints of goes

### Integration with [google/uuid](https://github.com/google/uuid)

goes presently integrates with [Google's UUID library](https://github.com/google/uuid)
to generate identifiers for [events](/guide/events/), [commands](/guide/commands/),
and [aggregates](/guide/aggregates/). This necessitates the adoption of UUIDs
throughout your application's domain model. The framework's reliance on UUIDs is
contingent on the current state of Go's type inference for type parameters.
As the Go language evolves, specifically with enhancements to type parameters,
there may be opportunities to generalize this requirement and accommodate
different identifier types.
