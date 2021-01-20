# Software Design Document

## Overview

This project aims to provide a progressively adaptable CQRS & Event Sourcing
framework for the Go language.

- Last updated: 2021/01/20
- Status: Planning
- Stakeholders:
  - [@bounoable](https://github.com/bounoable)

### Related articles

- https://cqrs.files.wordpress.com/2010/11/cqrs_documents.pdf
- https://martinfowler.com/bliki/CQRS.html
- https://www.youtube.com/watch?v=STKCRSUsyP0
- https://www.youtube.com/watch?v=LDW0QWie21s

## Context

### Problem

We need a replacement for the current CQRS/ES framework
([bounoable/cqrs-es](https://github.com/bounoable/cqrs-es)) we're using for some
projects. That project is more a prototype than a usable framework and is not
well tested.

### Goals

- progressive implementation
- as lightweight as possible
- avoid boilerplate code

### Non-goals

> TODO

## Design

The framework consists of the following modules:

- [event](./events.md) – publish & subscribe to (aggregate) events
- [aggregate](./aggregates.md) – create & manage aggregates
