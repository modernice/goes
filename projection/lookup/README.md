# Lookup

`lookup` maintains event-driven lookup tables. Events expose key/value pairs via
`ProvideLookup`, and the table can then be queried by aggregate name and id.

```go
 type UserRegistered struct{ Email string }
 func (e UserRegistered) ProvideLookup(p lookup.Provider) {
     p.Provide("email", e.Email)
 }
```

```go
l := lookup.New(store, bus, []string{"user_registered"})
errs, _ := l.Run(ctx)
<-l.Ready()
```
