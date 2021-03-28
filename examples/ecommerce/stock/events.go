package stock

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

const (
	Filled   = "stock.filled"
	Removed  = "stock.removed"
	Reserved = "stock.reserved"
	Released = "stock.released"
	Executed = "stock.executed"
)

type FilledEvent struct {
	Quantity int
}

type RemovedEvent struct {
	Quantity int
}

type ReservedEvent struct {
	Quantity int
	OrderID  uuid.UUID
}

type ReleasedEvent struct {
	OrderID uuid.UUID
}

type ExecutedEvent struct {
	OrderID uuid.UUID
}

func RegisterEvents(r event.Registry) {
	r.Register(Filled, func() event.Data { return FilledEvent{} })
	r.Register(Removed, func() event.Data { return RemovedEvent{} })
	r.Register(Reserved, func() event.Data { return ReservedEvent{} })
	r.Register(Released, func() event.Data { return ReleasedEvent{} })
	r.Register(Executed, func() event.Data { return ExecutedEvent{} })
}
