package todo

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
)

// ListRepository is the "todo list" repository.
//
// goes provides a generic TypedRepository that can be used to define your
// repositories within your own app, strongly typed. Use the
// github.com/modernice/goes/aggregate/repository.Typed function to create a
// TypedRepository from an aggregate.RepositoryOf[ID]
//
//	type List struct { *aggregate.Base }
//	func NewList(id uuid.UUID) *List { ... }
//
//	// Define the ListRepository interface as an alias.
//	type ListRepository = aggregate.TypedRepository[*List, uuid.UUID]
//
//	var repo aggregate.Repository
//	typed := repository.Typed(repo, NewList)
//
//	// typed is a ListRepository, which is an aggregate.TypedRepository[*List, uuid.UUID]
type ListRepository = aggregate.TypedRepository[*List, uuid.UUID]
