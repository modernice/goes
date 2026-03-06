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
  - icon: 🚀
    title: Event-Sourced Aggregates
    details: Embed a base type, register typed event handlers, and let the framework handle versioning, persistence, and replay.
  - icon: 🔌
    title: Distributed Events
    details: Publish and subscribe to events with NATS or use MongoDB and PostgreSQL as event stores. Swap backends without changing application code.
  - icon: 🧠
    title: Type-Safe Commands
    details: Dispatch and handle commands with full generic type safety. Synchronous or asynchronous — your choice.
  - icon: 🛠️
    title: Projection Toolkit
    details: Build read models with continuous or periodic schedules. Automatic progress tracking, debouncing, and startup catch-up.
  - icon: 📦
    title: Codec Registry
    details: Map event names to Go types for automatic serialization. JSON by default, swap to MessagePack or Protobuf with one option.
  - icon: 📸
    title: Snapshots
    details: Capture aggregate state at a point in time. Replay only recent events instead of the full history.
  - icon: ✅
    title: Testable
    details: Fluent test assertions for aggregates, in-memory backends for integration tests, and conformance suites for custom implementations.
  - icon: 🦾
    title: Modular Design
    details: Use only what you need — the event system, commands, projections — and adopt more components as your project grows.
  - icon: 🔋
    title: Batteries Included
    details: MongoDB, PostgreSQL, and NATS backends ready to go. In-memory implementations for testing. Zero external dependencies for prototyping.
---

<style>
:root {
  --vp-home-hero-name-color: transparent;
  --vp-home-hero-name-background: -webkit-linear-gradient(120deg, #38bdf8, #f0f9ff);
}
</style>
