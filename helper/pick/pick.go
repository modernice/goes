package pick

import (
	"github.com/google/uuid"
)

// An AggregateProvider has an Aggregate() (uuid.UUID, string, int) method.
// aggregate.Aggregate and event.Event implement this interface.
type AggregateProvider interface {
	Aggregate() (uuid.UUID, string, int)
}

// AggregateID returns the id that is returned by p.Aggregate().
func AggregateID(p AggregateProvider) uuid.UUID {
	id, _, _ := p.Aggregate()
	return id
}

// AggregateName returns the name that is returned by p.Aggregate().
func AggregateName(p AggregateProvider) string {
	_, name, _ := p.Aggregate()
	return name
}

// AggregateVersion returns the version that is returned by p.Aggregate().
func AggregateVersion(p AggregateProvider) int {
	_, _, version := p.Aggregate()
	return version
}
