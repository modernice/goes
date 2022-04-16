# Aggregates

Package `aggregate` provides the framework for building event-sourced aggregates.
It builds on top of the [event system](../event), so make sure to read the event
documentation first before reading further.

## Introduction

An aggregate is any type that implements the `Aggregate` interface:

```go
package aggregate

type Aggregate interface {
	// Aggregate returns the id, name and version of the aggregate.
	Aggregate() (uuid.UUID, string, int)

	// AggregateChanges returns the uncommited events of the aggregate.
	AggregateChanges() []event.Event

	// ApplyEvent applies an event onto the aggregate.
	ApplyEvent(event.Event)
}
```

You can either implement this interface by yourself or embed the `*Base` type.
Use the `New` function to initialize `*Base`:

```go
package example

// Foo is the "foo" aggregate.
type Foo struct {
	*aggregate.Base
}

// NewFoo returns the "foo" aggregate with the given id.
func NewFoo(id uuid.UUID) *Foo {
	return &Foo{
		Base: aggregate.New("foo", id),
	}
}
```

### Additional APIs

Aggregates can make use of additional, optional APIs provided by goes.
An aggregate that embeds `*Base` implements all of these APIs automatically:

- [`Aggregate`](./api.go)
- [`Committer`](./api.go)
- [`repository.ChangeDiscarder`](./repository/retry.go)
- [`snapshot.Aggregate`](./snapshot)

Read the documentation for each of these interfaces for more details.

## Aggregate events

An event-sourced aggregate transitions its state by applying events on itself.
Events are applied by the `ApplyEvent(event.Event)` method of the aggregate.
Here is a minimal "todo list" example:

```go
package todo

type List struct {
	*aggregate.Base

	Tasks []string
}

// NewList returns the todo list with the given id.
func NewList(id uuid.UUID) *User {
	return &List{Base: aggregate.New("list", id)}
}

func (l *List) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case "task_added":
		l.Tasks = append(l.Tasks, evt.Data().(string))
	case "task_removed":
		name := evt.Data().(string)
		for i, task := range l.Tasks {
			if task == name {
				l.Tasks = append(l.Tasks[:i], l.Tasks[i+1:]...)
				return
			}
		}
	}
}
```

The todo list now knows how to apply `"task_added"` and `"task_removed"` events.
What's missing are the commands to actually create the events and call the
`ApplyEvent` method with the created event:

```go
// ... previous code ...

// AddTask adds the given task to the list.
func (l *List) AddTask(task string) error {
	if l.Contains(task) {
		return fmt.Errorf("list already contains %q", task)
	}

	// aggregate.Next() creates the event and applies it using l.ApplyEvent()
	aggregate.Next(l, "task_added", task)

	return nil
}

// RemoveTask removes the given task from the list.
func (l *List) RemoveTask(task string) error {
	if !l.Contains(task) {
		return fmt.Errorf("list does not contain %q", task)
	}

	aggregate.Next(l, "task_removed", task)

	return nil
}

// Contains returns whether the list contains the given task.
func (l *List) Contains(task string) bool {
	task = strings.ToLower(task)
	for _, t := range l.Tasks {
		if strings.ToLower(t) == task {
			return true
		}
	}
	return false
}
```

That's it. Now you can create todo lists, add tasks, and remove them again:

```go
// ... previous code ...

func example() {
	list := NewList(uuid.New())

	if err := list.AddTask("do this and that"); err != nil {
		panic(fmt.Errorf("add task: %w", err))
	}

	if err := list.RemoveTask("do this and that"); err != nil {
		panic(fmt.Errorf("remove task: %w", err))
	}

	// list.AggregateVersion() == 2
	// list.AggregateChanges() returns two events – one "task_added" event
	// and one "task_removed" event.
}
```

## Generic helpers

Applying events within the `ApplyEvent` function is the most straightforward way
to implement an aggregate but can become quite messy if an aggregate consists of
many events.

goes provides type-safe, generic helpers that allow you to setup an event
applier function for each individual event. This is what the todo list example
looks like using generics:

```go
package todo

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

type List struct {
	*aggregate.Base

	Tasks []string
}

func NewList(id uuid.UUID) *List {
	l := &List{Base: aggregate.New("list", id)}

	event.ApplyWith(l, l.addTask, "task_added")
	event.ApplyWith(l, l.removeTask, "task_removed")

	return l
}

func (l *List) AddTask(task string) error { ... }
func (l *List) RemoveTask(task string) error { ... }

func (l *List) addTask(evt event.Of[string]) {
	l.Tasks = append(l.Tasks, evt.Data())
}

func (l *List) removeTask(evt event.Of[string]) {
	name := evt.Data()
	for i, task := range l.Tasks {
		if task == name {
			l.Tasks = append(l.Tasks[:i], l.Tasks[i+1:]...)
			return
		}
	}
}
```

## Testing

### TL;DR

Use the `test.Change()` and `test.NoChange()` testing helpers to ensure correct
implementation of aggregate methods.

```go
package todo_test

import (
	"github.com/modernice/goes/test"
)

func TestNewList(t *testing.T) {
	// Test that todo.NewList() returns a valid aggregate.
	test.NewAggregate(t, todo.NewList, "list")
}

func TestXXX(t *testing.T) {
	// Aggregate should have applied and recorded the given event.
	test.Change(t, foo, "<event-name>")

	// Aggregate should have applied and recorded the given event with
	// the given event data.
	test.Change(t, foo, "<event-name>", test.EventData(<event-data>))

	// Aggregate should have applied and recorded the given event with
	// the given event data exactly 3 times.
	test.Change(
		t, foo, "<event-name>",
		test.EventData(<event-data>),
		test.Exactly(3),
	)

	// Aggregate should NOT have applied and recorded the given event.
	test.NoChange(t, foo, "<event-name>")

	// Aggregate should NOT have applied and recorded the given event with
	// the given event data.
	test.NoChange(t, foo, "<event-name>", test.EventData(<event-data>))
}
```

Testing of aggregates can become error-prone if one forgets to consider that
aggregates are event-sourced. Take a look at this example:

```go
package todo_test

func TestList_AddTask(t *testing.T) {
	l := todo.NewList(uuid.New())

	if l.Contains("foo") {
		t.Fatalf("list should not contain %q until added", "foo")
	}

	if err := l.AddTask("foo"); err != nil {
		t.Fatalf("failed to add task %q", "foo")
	}

	if !l.Contains("foo") {
		t.Fatalf("list should contain %q after adding", "foo")
	}
}
```

Even if the above test suceeds, it does not guarantee that the aggregate was
implemented correctly. The following `AddTask` implementation bypasses the
indirection through the `ApplyEvent` method and updates the state directly,
resulting in a passing test even though the aggregate would behave incorrectly
when used in goes' components.

```go
package todo

func (l *List) AddTask(task string) error {
	l.Tasks = append(l.Tasks, task)
}
```

To circumvent this issue, goes provides helpers to test _aggregate changes_.
The above test would be rewritten as:

```go
package todo_test

import "github.com/modernice/goes/test"

func TestList_AddTask(t *testing.T) {
	l := todo.NewList(uuid.New())

	if l.Contains("foo") {
		t.Fatalf("list should not contain %q until added", "foo")
	}

	if err := l.AddTask("foo"); err != nil {
		t.Fatalf("failed to add task %q", "foo")
	}

	if !l.Contains("foo") {
		t.Fatalf("list should contain %q after adding", "foo")
	}

	test.Change(t, l, "task_added", test.EventData("foo"))
}
```

The `test.Change()` helper checks if the aggregate has recorded a `"task_added"`
change with `"foo"` as the event data.

## Persistence

The `Repository` type defines an aggregate repository that allows you to save
and fetch aggregates to and from an underlying event store:

```go
package aggregate

type Repository interface {
	Save(ctx context.Context, a Aggregate) error
	Fetch(ctx context.Context, a Aggregate) error
	FetchVersion(ctx context.Context, a Aggregate, v int) error
	Query(ctx context.Context, q Query) (<-chan History, <-chan error, error)
	Use(ctx context.Context, a Aggregate, fn func() error) error
	Delete(ctx context.Context, a Aggregate) error
}
```

The implementation of this repository can be found in the `repository` package.
Use `repository.New` to create a repository from an event store:

```go
package example

import (
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event"
)

func example(store event.Store) {
	repo := repository.New(store)
}
```

### Save an aggregate

```go
package example

func example(repo aggregate.Repository) {
	l := todo.NewList(uuid.New())
	l.AddTask("foo")
	l.AddTask("bar")
	l.AddTask("baz")

	if err := repo.Save(context.TODO(), l); err != nil {
		panic(fmt.Errorf("save todo list: %w", err))
	}
}
```

### Fetch an aggregate

In order to fetch an aggregate, it must be passed to `Repository.Fetch()`.
The repository fetches and applies the event stream of the aggregate to
reconstruct its current state.

An aggregate does not need to have an event stream to be fetched; if an
aggregate has no events, `Repository.Fetch()` is a no-op.

Fetching an aggregate multiple times is also not a problem because the
repository will only fetch and apply events that haven't been applied yet.
This also means that `Repository.Fetch()` can be used to "refresh" an aggregate
– to get to its most current state without fetching unnecessary events.

```go
package example

func example(repo aggregate.Repository) {
	l := todo.NewList(uuid.New())

	if err := repo.Fetch(context.TODO(), l); err != nil {
		panic(fmt.Errorf(
			"fetch todo list: %w [id=%s]", err, l.AggregateID(),
		))
	}
}
```

You can also fetch a specific version of an aggregate, ignoring all events with
a version higher than the provided version:

```go
package example

func example(repo aggregate.Repository) {
	l := todo.NewList(uuid.New())

	if err := repo.FetchVersion(context.TODO(), l, 5); err != nil {
		panic(fmt.Errorf(
			"fetch todo list at version %d: %w [id=%s]",
			5, err, l.AggregateID(),
		))
	}
}
```

### "Use" an aggregate

`Repository.Use()` is a convenience method to fetch an aggregate, "use" it, and
then insert new changes into the event store:

```go
package example

func example(repo aggregate.Repository) {
	l := todo.NewList(uuid.New())

	if err := repo.Use(context.TODO(), l, func() error {
		return l.AddTask("foo")
	}); err != nil {
		panic(err)
	}
}
```

### Delete an aggregate

Hard-deleting aggregates should be avoided because that can lead to esoteric
issues that are hard to debug. Consider using [soft-deletes](
#soft-delete-an-aggregate) instead.

To delete an aggregate, the repository deletes its event stream from the event
store:

```go
package example

func example(repo aggregate.Repository) {
	l := todo.NewList(uuid.New())

	if err := repo.Delete(context.TODO(), l); err != nil {
		panic(fmt.Errorf(
			"delete todo list: %w [id=%s]", err, l.AggregateID(),
		))
	}
}
```

### Soft-delete an aggregate

Soft-deleted aggregates cannot be fetched and are excluded from query results of
aggregate repositories. In order to soft-delete an aggregate, a specific event
that flags the aggregate as soft-deleted must be inserted into the event store.
The event must have event data that implements the `SoftDeleter` interface:

```go
package example

type DeletedData struct {}

func (DeletedData) SoftDelete() bool { return true }

func example() {
	evt := event.New("deleted", DeletedData{}, event.Aggregate(...))
}
```

If the event stream of an aggregate contains such an event, the aggregate is
considered to be soft-deleted and will be excluded from query results of the
aggregate repository. Additionally, the `Repository.Fetch()` method will return
`repository.ErrDeleted` for the aggregate.

Soft-deleted aggregates can also be restored by inserting an event with event
data that implements `SoftRestorer`:

```go
package example

type RestoredData struct {}

func (RestoredData) SoftRestore() bool { return true }

func example() {
	evt := event.New("restored", RestoredData{}, event.Aggregate(...))
}
```

### Query aggregates

Aggregates can be queried from the event store. When queried, the repository
returns a `History` channel (lol) and an `error` channel. A `History` can be
applied onto an aggregate to reconstruct its current state.

```go
package example

import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/query"
	"github.com/modernice/goes/helper/streams"
)

func example(repo aggregate.Repository) {
	res, errs, err := repo.Query(context.TODO(), query.New(
		// Query "foo", "bar", and "baz" aggregates.
		query.Name("foo", "bar", "baz"),

		// Query aggregates that have one of the provided ids.
		query.ID(uuid.UUID{...}, uuid.UUID{...}),
	))

	if err := streams.Walk(
		context.TODO(),
		func(his aggregate.History) error {
			log.Printf(
				"Name: %s ID: %s",
				his.AggregateName(),
				his.AggregateID(),
			)

			var foo aggregate.Aggregate // fetch the aggregate
			his.Apply(foo) // apply the history
		},
		res,
		errs,
	); err != nil {
		panic(err)
	}
}
```

### Typed repositories

The `Repository` interface defines a generic aggregate repository for all kinds
of aggregates. The `TypedRepository` can be used to define a type-safe
repository for a specific aggregate. The `TypedRepository` removes the need for
passing the aggregate instance to repository methods.

To create a type-safe repository for an aggregate, pass the `Repository` and the
constructor of the aggregate to `repository.Typed()`:

```go
package todo

import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/event"
)

// List is the "todo list" aggregate.
type List struct { *aggregate.Base }

// ListRepository is the "todo list" repository.
type ListRepository = aggregate.TypedRepository[*List]

// NewList returns the "todo list" with the given id.
func NewList(id uuid.UUID) *List {
	return &List{Base: aggregate.New("list", id)}
}

// NewListRepository returns the "todo list" repository.
func NewListRepository(repo aggregate.Repository) ListRepository {
	return repository.Typed(repo, NewList)
}

func example(store event.Store) {
	repo := repository.New(store)
	lists := NewListRepository(repo)

	// Fetch a todo list by id.
	l, err := lists.Fetch(context.TODO(), uuid.New())
	if err != nil {
		panic(fmt.Errorf("fetch list: %w", err))
	}
	// l is a *List

	// "Use" a list by id.
	if err := lists.Use(context.TODO(), uuid.New(), func(l *List) error {
		return l.AddTask("foo")
	}); err != nil {
		panic(fmt.Errof("use list: %w", err))
	}

	// The TypedRepository will only ever return *List aggregates.
	// All other aggregates that would be returned by the passed query,
	// are simply discarded from the result.
	res, errs, err := lists.Query(context.TODO(), query.New(...))
}
```
