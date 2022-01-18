package pick

import "github.com/google/uuid"

type AggregateProvider interface {
	Aggregate() (uuid.UUID, string, int)
}

// AggregateID returns the id that is returned by p.Aggregate().
func AggregateID[Provider AggregateProvider](p Provider) uuid.UUID {
	id, _, _ := p.Aggregate()
	return id
}

// AggregateName returns the name that is returned by p.Aggregate().
func AggregateName[Provider AggregateProvider](p Provider) string {
	_, name, _ := p.Aggregate()
	return name
}

// AggregateVersion returns the version that is returned by p.Aggregate().
func AggregateVersion[Provider AggregateProvider](p Provider) int {
	_, _, version := p.Aggregate()
	return version
}
