# Lookups

The `lookup` package provides an event-driven lookup table for your aggregates.

## Example: User Signup

Lookup tables are often needed to find a specific aggregate by a field other
than its UUID. An example for this is a user signup where an email address must
be unique across all users.

### Events

```go
package auth

const UserRegistered = "auth.user.registered"
const EmailLookup = "email"

type UserRegisteredData struct {
	Email string
}

func (data UserRegisteredData) ProvideLookup(p lookup.Provider) {
	p.Provide(EmailLookup, data.Email)
}
```

### User

```go
package auth

const UserAggregate = "auth.user"

type User struct {
	*aggregate.Base

	Name string
	Email string
}

func NewUser(id uuid.UUID) *User {
	u := &User{Base: aggregate.New(UserAggregate, id)}

	aggregate.ApplyWith(u, UserRegistered, u.register)

	return u
}

func (u *User) Register(
	ctx context.Context,
	emails *lookup.Lookup,
	email string,
) error {
	if lookup.Contains[string](
		ctx, emails, UserAggregate, EmailLookup, u.AggregateID(),
	) {
		return errors.New("email address is already in use")
	}

	aggregate.NextEvent(u, UserRegistered, email)
}

func (u *User) register(e event.Of[string]) {
	u.Email = e.Data()
}
```

### Run

```go
package example

func registerUser(email string) {
	var store event.Store
	var bus event.Bus

	l := lookup.New(store, bus, []string{UserRegistered})
	
	errs, err := l.Run(context.TODO())
	if err != nil {
		panic(err)
	}

	<-l.Ready()

	u := NewUser(uuid.New())
	if err := u.Register(ctx, emails, email); err != nil {
		panic(err)
	}
}
```
