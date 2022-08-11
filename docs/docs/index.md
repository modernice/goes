---
layout: home
hero:
  name: goes
  text: Event-Sourcing Framework
  tagline: Build distributed, event-driven applications in Go.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/introduction
    - theme: alt
      text: Code Examples
      link: /examples/
    - theme: alt
      text: GitHub
      link: https://github.com/modernice/goes
features:
  - icon: 🔌
    title: Event System
    details: A distributed and composable event system, which all other components build on.
  - icon: 🧠
    title: Command System
    details: Dispatch and handle commands between multiple (distributed) services.
  - icon: 🚀
    title: Aggregate Framework
    details: Easily make your aggregates event-sourced using the provided utility functions.
  - icon: 🛠️
    title: Projection Toolkit
    details: Build, schedule, and run projections in a distributed system.
  - icon: 🛠️
    title: Process Managers
    details: Orchestrate complex inter-service transactions. (soon)
  - icon: 🔋
    title: Batteries Included
    details: Multiple, ready-to-use backend integrations (MongoDB, Postgres, NATS)
  - icon: ✅
    title: Testable
    details: Testing utilities are provided for all components. (soon)
  - icon: ️🦾
    title: Modular Design
    details: Use only what you need (e.g. the event system), and incrementally adopt components whenever the need arises.
  - icon: ⚡️
    title: Fast & Low-Memory
    details: goes' components send data in streams, not as slices. This keeps the memory footprint low, especially when working with large streams of events.
---
