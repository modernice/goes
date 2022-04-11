# Best Practices

This is a living document where I list best practices that I discovered while
working with this library. These will be integrated into the documentation someday.

## General

- keep aggregates small (as few events as possible)
	- to keep their event stream small
- do validation within the aggregates before raising an event, not within the
	event applier
	- better performance when re-building the aggregate
	- event appliers are not responsible for validation, they just apply "facts"
- do not raise *unnecessary* aggregate events that don't change the state of
	the aggregate

## Projections

- keep projections performant
	- run costly computations in a `finalize` method after applying a projection
		job onto a projection
	- generally, don't call long-running functions that require a
		`context.Context` within event appliers.
