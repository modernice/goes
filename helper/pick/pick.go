package pick

import (
	"github.com/modernice/goes"
)

// An AggregateProvider has an Aggregate() (ID, string, int) method.
// aggregate.Aggrgeate and event.Of[any, uuid.UUID] implement this interface.
type AggregateProvider[ID goes.ID] interface {
	Aggregate() (ID, string, int)
}

// AggregateID returns the id that is returned by p.Aggregate().
func AggregateID[ID goes.ID](p AggregateProvider[ID]) ID {
	id, _, _ := p.Aggregate()
	return id
}

// AggregateName returns the name that is returned by p.Aggregate().
func AggregateName[ID goes.ID](p AggregateProvider[ID]) string {
	_, name, _ := p.Aggregate()
	return name
}

// AggregateVersion returns the version that is returned by p.Aggregate().
func AggregateVersion[ID goes.ID](p AggregateProvider[ID]) int {
	_, _, version := p.Aggregate()
	return version
}
