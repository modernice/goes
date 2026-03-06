# Architecture

::: warning Work in Progress
This guide is being written.
:::

This guide will cover:

- How all pieces fit together
- The write path: Command → Command Bus → Handler → Aggregate → Events → Store + Bus
- The read path: Event Bus → Schedule → Job → Projection
- Key interfaces: `event.Store`, `event.Bus`, `aggregate.Repository`, `command.Bus`
- The streaming pattern — why goes uses channels
- Dependency graph
