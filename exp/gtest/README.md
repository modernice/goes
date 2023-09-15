# Testing Utilities

> This is an experimental package for testing utilities. **The API may change at any time.**

The `gtest` package will provide a suite of tools for easily testing
event-sourcing patterns. Currently, you can:

- Test aggregate constructors.
- Ensure aggregates produce the expected events.
- Check for unexpected events from aggregates.

## Usage

Consider an aggregate `User`:

```go
package auth

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

type User struct {
	*aggregate.Base

	Username string
	Age      int
}

func NewUser(id uuid.UUID) *User {
	user := &User{Base: aggregate.New("auth.user", id)}

	event.ApplyWith(user, user.created, "auth.user.created")

	return user
}

type UserCreation struct {
	Username string
	Age      int
}

func (u *User) Create(username string, age int) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if age < 0 {
		return fmt.Errorf("age cannot be negative")
	}

	aggregate.Next(u, "auth.user.created", UserCreation{
		Username: username,
		Age:      age,
	})

	return nil
}

func (u *User) created(e event.Of[UserCreation]) {
	u.Username = e.Username
	u.Age = e.Age
}
```

Using `gtest`, you can efficiently test this aggregate.

### Testing Constructors

To ensure the `NewUser` constructor returns a valid `User` with the correct
AggregateID:

```go
func TestNewUser(t *testing.T) {
	gtest.Constructor(auth.NewUser).Run(t)
}
```

### Testing Aggregate Transitions

To ensure the `User` aggregate correctly transitions to the `auth.user.created`
event with expected data:

```go
func TestUser_Create(t *testing.T) {
	u := auth.NewUser(uuid.New())

	if err := u.Create("Alice", 25); err != nil {
		t.Errorf("Create failed: %v", err)
	}

	gtest.Transition(
		"auth.user.created",
		UserCreation{Username: "Alice", Age: 25},
	).Run(t, u)

	if u.Username != "Alice" {
		t.Errorf("expected Username to be %q; got %q", "Alice", u.Username)
	}

	if u.Age != 25 {
		t.Errorf("expected Age to be %d; got %d", 25, u.Age)
	}
}
```

### Testing Signals (events without data)

To ensure the `User` aggregate correctly transitions to the `auth.user.deleted`
event:

```go
func TestUser_Delete(t *testing.T) {
	u := auth.NewUser(uuid.New())

	if err := u.Delete(); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	gtest.Signal("auth.user.deleted").Run(t, u)
}
```

### Testing Non-Transitions

To ensure a specific event (e.g., `auth.user.created`) is not emitted by the
`User` aggregate:

```go
func TestUser_Create_negativeAge(t *testing.T) {
	u := auth.NewUser(uuid.New())

	if err := u.Create("Alice", -3); err == nil {
		t.Errorf("Create should fail with negative age")
	}

	gtest.NonTransition("auth.user.created").Run(t, u)
}
```
