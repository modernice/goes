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
- 
