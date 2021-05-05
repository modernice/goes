> Documentation is work in progress.

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

# Example - To-Do app

In this guide we're going to build a simple To-Do app. But to make things
interesting, we're going overkill and make it distributed and event-sourced.

We split the app into 3 microservices:

- the `task` service provides the functionality to create, rename, delete and
complete tasks
- the `notification` service sends notifications about due tasks and tasks that
were completed
- the `dashboard` service generates and provides analytics for the tasks

## Installation

First, initialize a new project:

```sh
mkdir todo && cd todo && go mod init todo
```

Then install goes:

```sh
go get github.com/modernice/goes
```

## Task service

### Define Events

Define the Events for the `Task` Aggregate and allow them to be registered in
an Event Registry:

```go
// task/events.go

package task

import "github.com/modernice/goes/event"

const (
    // Created is the Event name for creating a Task.
    Created = "task.created"

    // Renamed is the Event name for renaming a Task.
    Renamed = "task.renamed"

    // Deleted is the Event name for deleting a Task.
    Deleted = "task.deleted"

    // Completed is the Event name for completing a Task.
    Completed = "task.completed"
)

// CreatedEvent is the Event Data creating a Task.
type CreatedEvent struct {
    Title string
    DueAt time.Time
}

// RenamedEvent is the Event Data for renaming a Task.
type RenamedEvent struct {
    OldTitle string
    NewTitle string
}

// CompletedEvent is the Event Data for completing a Task.
type CompletedEvent struct{}

// DeletedEvent is the Event Data for deleting a Task.
type DeletedEvent struct {}

// RegisterEvents registers the Task Events into an Event Registry so that the
// Registry can constructs the Events.
func RegisterEvent(r event.Registry) {
    r.Register(Created, func() event.Data { return CreatedEvent{} })
    r.Register(Renamed, func() event.Data { return RenamedEvent{} })
    r.Register(Deleted, func() event.Data { return DeletedEvent{} })
    r.Register(Completed, func() event.Data { return CompletedEvent{} })
}
```

### Create Task Aggregate

```go
// task/task.go

package task

import (
    "strings"
    "time"

    "github.com/modernice/goes/aggregate"
    "github.com/modernice/goes/event"
)

var (
    // ErrEmptyTitle is returned when trying to create a Task with an empty
    // title.
    ErrEmptyTitle = errors.New("empty title")

    // ErrInvalidDueDate is returned when trying to create a Task with a due
    // date before time.Now().
    ErrInvalidDueDate = errors.New("invalid due date")

    // ErrAlreadyCreated is returned when trying to create an already created
    // Task.
    ErrAlreadyCreated = errors.New("Task already created")

    // ErrAlreadyCompleted is returned when trying to complete an already
    // completed Task.
    ErrAlreadyCompleted = errors.New("Task already completed")

    // ErrDeleted is returned when trying to act on a deleted Task.
    ErrDeleted = errors.New("Task deleted")
)

// AggregateName is the name of the Task Aggregate.
const AggregateName = "task"

type Task struct {
    // embed *aggregate.Base so we don't have to implement the Aggreate interface
    // by ourself
    *aggregate.Base

    Title       string
    DueAt       time.Time
    DeletedAt   time.Time
    CompletedAt time.Time
}

// New returns the Task with the given UUID.
func New(id uuid.UUID) *Task {
    return &Task{
        Base: aggregate.New(AggregateName, id),
    }
}

// ApplyEvent implements aggregate.Aggregate.
func (t *Task) ApplyEvent(evt event.Event) {
    switch evt.Name() {
    case Created:
        t.create(evt)
    case Renamed:
        t.rename(evt)
    case Deleted:
        t.delete(evt)
    case Completed:
        t.complete(evt)
    }
}

// Create creates the Task. Note that the actual instantiation of the Task
// happens in NewTask, but Create actually "creates" the Task with validation
// if inputs.
func (t *Task) Create(title string, dueAt time.Time) error {
    // If the title of the Task is not empty, it was already created.
    if t.Title != "" {
        return ErrAlreadyCreated
    }

    // Validate that the title isn't empty.
    title, err := validateTitle(title)
    if err != nil {
        return err
    }

    if dueAt.Before(time.Now()) {
        return ErrInvalidDueDate
    }

    // Here we create and apply the next Event for the Task.
    // aggregate.NextEvent automatically sets the correct Aggregate information
    // for the Event based on the current state of the Task.
    // The Event is directly applied on the Task by calling t.ApplyEvent with
    // the created Event.
    aggregate.NextEvent(t, Created, CreatedEvent{
        Title: title,
        DueAt: dueAt,
    })

    return nil
}

// create actually applies the "task.created" Event that is created in t.Create.
func (t *Task) create(evt event.Event) {
    data := evt.Data().(CreatedEvent)

    t.Title = data.Title
    t.DueAt = data.DueAt
}

func (t *Task) Rename(title string) error {
    title, err := validateTitle(title)
    if err != nil {
        return err
    }

    aggregate.NextEvent(t, Renamed, RenamedEvent{
        OldTitle: t.Title,
        NewTitle: title,
    })

    return nil
}

func (t *Task) rename(evt event.Event) {
    data := evt.Data().(RenamedEvent)
    t.Title = t.NewTitle
}

func (t *Task) Delete() error {
    if !t.DeletedAt.IsZero() {
        return ErrDeleted
    }

    aggregate.NextEvent(t, Deleted, DeletedEvent{})

    return nil
}

func (t *Task) delete(evt event.Event) {
    t.DeletedAt = evt.Time()
}

func (t *Task) Complete() error {
    if !t.CompletedAt.IsZero() {
        return ErrAlreadyCompleted
    }

    if !t.DeletedAt.IsZero() {
        return ErrDeleted
    }

    aggregate.NextEvent(t, Completed, CompletedEvent{})

    return nil
}

func (t *Task) complete(evt event.Event) {
    t.CompletedAt = evt.Time()
}

func validateTitle(title string) (string, error) {
    title = strings.TrimSpace(title)
    if title == "" {
        return title, ErrEmptyTitle
    }
    return title, nil
}
```

### Define Commands

Then define the commands for the Tasks and allow them to be registered in a
Command Registry:

```go
// task/commands.go

package task

import "github.com/modernice/goes/command"

// Just like with Events, define constants for each Command name.
const 
    Create = "task.create"
    Rename = "task.rename"
    Delete = "task.delete"
    Complete = "task.complete"
)

// CreatePayload is the Command Payload for creating a Task.
type CreatePayload struct {
    Title string
    DueAt time.Time
}

// RenamePayload is the Command Payload for renaming a Task.
type RenamePayload struct {
    Title string
}

// DeletePayload is the Command Payload for deleting a Task.
type DeletePayload struct{}

// CompletePayload is the Command Payload for completing a Task.
type CompletePayload struct{}

// Create returns the Command to create a Task with the given UUID, title and
// due date.
func Create(id uuid.UUID, title string, dueAt time.Time) command.Command {
    return command.New(
        Create,
        CreatePayload{
            Title: title,
            DueAt: dueAt,
        },
        // add Aggregate data to the Command
        command.Aggregate(AggregateName, id), 
    )
}

// Rename returns the Command to rename the Task with the given UUID with the
// provided title.
func Rename(id uuid.UUID, title string) command.Command {
    return command.New(
        Rename,
        RenamePayload{Title: title},
        command.Aggregate(AggregateName, id),
    )
}

// Delete returns the Command to delete the Task with the given UUID.
func Delete(id uuid.UUID) command.Command {
    return command.New(
        Delete,
        DeletePayload{},
        command.Aggregate(AggregateName, id),
    )
}

// Complete returns the Command to complete the Task with the given UUID.
func Complete(id uuid.UUID) command.Command {
    return command.New(
        Complete,
        CompletePayload{},
        command.Aggregate(AggregatName, id),
    )
}

// RegisterCommands registers the Task Commands into a Command Registry.
func RegisterCommands(r command.Registry) {
    r.Register(Create, func() command.Payload { return CreatePayload{} })
    r.Register(Rename, func() command.Payload { return RenamePayload{} })
    r.Register(Delete, func() command.Payload { return DeletePayload{} })
    r.Register(Complete, func() command.Payload { return CompletePayload{} })
}
```

### Command handling

In order for the Task service to handle Commands we need to write handlers for
the all Commands:

```go
// task/commands.go

package task

import (
    ...

    "github.com/modernice/goes/command"
    "github.com/modernice/goes/helper/fanin"
)

// still in commands.go, Commands are defined above

// HandleCommands handles Task Commands until ctx is canceled.
func HandleCommands(
    ctx context.Context,
    bus command.Bus,
    repo aggregate.Repository,
) (<-chan error, error) {
    // command.NewHandler returns a convenient helper that allows us to easily
    // register Command handlers through the provided Command Bus
    h := command.NewHandler(bus)

    createErrors, err := h.Handle(ctx, Create, func(ctx command.Context) error {
        cmd := ctx.Command()
        load := cmd.Payload().(CreatePayload)

        // We create the Task with the UUID from the Command.
        t := New(cmd.AggregateID())

        // Then we fetch and apply the Events of the Task.
        // Fetching a Task that doesn't exist yet is basically a no-op because
        // if no Events for the Task exist yet, nothing is applied to the Task.
        // This means that Fetch doesn't return an error if the Task doesn't
        // exist yet, which is pretty convenient.
        if err := repo.Fetch(ctx, t); err != nil {
            return fmt.Error("fetch Task: %w", err)
        }

        // Here we actually "create" the Task by using calling Create on the
        // actual Task.
        if err := t.Create(load.Title, load.DueAt); err != nil {
            return err
        }

        // Save the changes (Events) of the Task into the Aggregate Repository.
        if err := repo.Save(ctx, t); err != nil {
            return fmt.Errorf("save Task: %w", err)
        }

        return nil
    })

    renameErrors, err := h.Handle(ctx, Rename, func(ctx command.Context) error {
        cmd := ctx.Command()
        load := cmd.Payload().(RenamePayload)

        t := New(cmd.AggregateID())
        if err := repo.Fetch(ctx, t); err != nil {
            return fmt.Errorf("fetch Task: %w", err)
        }

        if err := t.Rename(load.Title); err != nil {
            return err
        }

        if err := repo.Save(ctx, t); err != nil {
            return fmt.Errorf("save Task: %w", err)
        }

        return nil
    })

    deleteErrors, err := h.Handle(ctx, Delete, func(ctx command.Context) error {
        cmd := ctx.Command()

        t := New(cmd.AggregateID())
        if err := repo.Fetch(ctx, t); err != nil {
            return fmt.Errorf("fetch Task: %w", err)
        }

        if err := t.Delete(); err != nil {
            return err
        }

        if err := repo.Save(ctx, t); err != nil {
            return fmt.Errorf("save Task: %w", err)
        }

        return nil
    })

    completeErrors, err := h.Handle(ctx, Complete, func(ctx command.Context) error {
        cmd := ctx.Command()

        t := New(cmd.AggregateID())
        if err := repo.Fetch(ctx, t); err != nil {
            return fmt.Errorf("fetch Task: %w", err)
        }

        if err := t.Complete(); err != nil {
            return err
        }

        if err := repo.Save(ctx, t); err != nil {
            return fmt.Errorf("save Task: %w", err)
        }

        return nil
    })

    // Use the `fanin` tool to return a single error channel from multiple ones.
    out, stop := fanin.Errors(
        createErrors,
        renameErrors,
        deleteErrors,
        completeErrors,
    )

    go func(){
        <-ctx.Done()
        stop()
    }()

    return out, nil
}
```

### Create the main.go

Now we just need to wire the Task service together in a main.go:

```go
// cmd/task/main.go

package main

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    // TBD
}
```
