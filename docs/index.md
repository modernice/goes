# Introduction

## What can I do with goes?

`goes` is a toolkit for creating distributed and event-sourced applications. You
focus on writing domain logic and goes wires your services together with as little
boilerplate code as possible.

If you take a look at the [ecommerce example](../examples/ecommerce) code, you
can see an example microservice stack with `Product`, `Stock` and `Order`
services, each representing a domain boundary, orchestrated together with a SAGA
that dispatched commands to each of the microservices, all **without having to write a
single RPC/JSON/HTTP server**.

## Getting started

## Installation

```sh
go get github.com/modernice/goes
```
