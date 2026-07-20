# TODO

- [ ] Change `event.NewRegistry()` to `codec.New()` (saga service default encoding)
- [ ] Remove `Strict` option from `saga.Config`
- [ ] Optimize trigger replay by only fetching `SagaStarted` and `SagaCompleted` events first
- [ ] Rewrite "Event handler" docs page (`docs/guide/event-handlers.md`)
- [ ] Check "Lookups" docs page (`docs/guide/lookups.md`)
- [ ] Check "Cookbook" docs page (`docs/reference/cookbook.md`)
- [ ] Rework "Aggregate-Owned Command Handlers" docs section
