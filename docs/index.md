---
layout: home

hero:
  name: goes
  text: Event-Sourcing Framework for Go
  tagline: Build distributed, event-sourced applications with type-safe aggregates, commands, and projections.
  actions:
    - theme: brand
      text: Start Tutorial
      link: /tutorial/
    - theme: alt
      text: Quick Start
      link: /getting-started/quick-start
    - theme: alt
      text: GitHub
      link: https://github.com/modernice/goes

features:
  - title: Event-Sourced Aggregates
    details: Embed a base type, register typed event handlers, and let the framework handle versioning, persistence, and replay.
  - title: Distributed Events
    details: Publish and subscribe to events with NATS or use MongoDB and PostgreSQL as event stores. Swap backends without changing application code.
  - title: Type-Safe Commands
    details: Dispatch and handle commands with full generic type safety. Synchronous or asynchronous — your choice.
  - title: Projection Toolkit
    details: Build read models with continuous or periodic schedules. Automatic progress tracking, debouncing, and startup catch-up.
  - title: Batteries Included
    details: MongoDB, PostgreSQL, and NATS backends ready to go. In-memory implementations for testing. Zero external dependencies for prototyping.
---
