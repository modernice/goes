# Sagas

::: warning Deprecated
The `saga` package has been superseded by the `workflow` package and will be removed in a future release. This page has moved:

**→ [Workflows](/guide/workflows)** — the guide to the durable, event-driven workflow runtime that replaces sagas.
:::

In goes, multi-step business processes — sagas, process managers — are modeled with the [`workflow` package](/guide/workflows). Workflow instances are event-sourced aggregates: every step is persisted, commands and timeouts are recorded as durable effects, and any service instance can recover and resume the work of a crashed one.

The legacy `saga` package implemented an **in-process** SAGA coordinator: it ran a predefined sequence of actions within a single process and compensated completed actions in reverse order when a later action failed. Because execution was neither persisted nor distributed, a crashed process could not recover a running SAGA — the limitation that motivated the replacement.

If you are migrating existing code, see [Migrating from the saga Package](/guide/workflows#migrating-from-the-saga-package) for a side-by-side mapping of the two APIs.
