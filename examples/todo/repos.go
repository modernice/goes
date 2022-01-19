package todo

import "github.com/modernice/goes/aggregate"

// ListRepository is the "todo list" repository.
type ListRepository = aggregate.TypedRepository[*List]
