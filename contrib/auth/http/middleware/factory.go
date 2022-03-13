package middleware

import (
	"net/http"

	"github.com/modernice/goes/aggregate"
)

// Factory is the middleware factory. It is not required to be used but it
// allows to create middleware without having to pass the PermissionRepository
// and Lookup.
type Factory struct {
	perms  PermissionFetcher
	lookup Lookup
}

// NewFactory returns a new middleware factory.
func NewFactory(perms PermissionFetcher, lookup Lookup) Factory {
	return Factory{
		perms:  perms,
		lookup: lookup,
	}
}

// Authorize returns the Authorize middleware.
func (f Factory) Authorize(authorize func(Authorizer, *http.Request)) func(http.Handler) http.Handler {
	return Authorize(f.lookup, authorize)
}

// AuthorizeField returns the AuthorizeField middleware.
func (f Factory) AuthorizeField(field string) func(http.Handler) http.Handler {
	return AuthorizeField(field)
}

// Permission returns the Permission middleware.
func (f Factory) Permission(action string, extractRef func(*http.Request) aggregate.Ref) func(http.Handler) http.Handler {
	return Permission(f.perms, action, extractRef)
}

// PermissionField returns the PermissionField middleware.
func (f Factory) PermissionField(action, aggregateName, field string) func(http.Handler) http.Handler {
	return PermissionField(f.perms, action, aggregateName, field)
}
