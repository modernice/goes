# Introduction

**goes** is a framework for building distributed, event-driven applications in Go.

This documentation assumes knowledge of [CQRS](
https://martinfowler.com/bliki/CQRS.html), [event-sourcing](
https://martinfowler.com/eaaDev/EventSourcing.html), and other
[related concepts](https://github.com/heynickc/awesome-ddd). Please make
yourself familiar with these before reading further.

::: warning Work in progress
This documentation is work-in-progress and very likely lacks important
information. Please [open an issue](https://github.com/modernice/goes/issues)
or [start a discussion](https://github.com/modernice/goes/discussions/new) if
you can't find what you're looking for.
:::

## Motivation

Building event-sourced applications is a difficult task, especially when adding
distributiveness to the system. You have to think about and implement things
like "eventual consistency" and "optimistic concurrency", which can be challenging
and can lead to hard-to-debug errors (especially in production environments).
Much tooling is required to 
You quickly realize that you 

goes tries to provide the basic building blocks that are required by (nearly)
all event-driven applications. Additional, flexible tooling allows you to solve
common event-sourcing problems with ease.

## Limitations

### Hard-dependency on [google/uuid](https://github.com/google/uuid)

goes has a hard-dependency on [Google's UUID library](https://github.com/google/uuid)
for the ids of [events](/guide/events/), [commands](/guide/commands/), and
[aggregates](/guide/aggregates/). This requires you to also use UUIDs within
your application. Should Go's type inference for type parameters improve in the
future, this dependency may be removed to allow for any type to be used as an id.
