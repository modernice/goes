package clitest

import (
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus/chanbus"
	"github.com/modernice/goes/event/eventstore/memstore"
)

func SetupEvents() (event.Registry, event.Bus, event.Store) {
	reg := event.NewRegistry()
	bus := chanbus.New()
	store := memstore.New()
	return reg, bus, store
}
