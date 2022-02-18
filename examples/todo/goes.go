package todo

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

type Event[Data any] event.Of[Data, uuid.UUID]
