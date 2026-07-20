package saga

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal"
)

// Correlator finds the saga instance that should process a trigger event.
type Correlator[Data any] func(event.Of[Data]) (uuid.UUID, bool)

// Definition is the static definition of a saga type.
type Definition struct {
	def *definition
}

// Registration mutates a saga definition.
type Registration[S Instance] struct {
	apply func(*definition)
}

type definition struct {
	name          string
	new           func(uuid.UUID) Instance
	registrations map[string][]triggerRegistration
}

type triggerPhase uint8

const (
	phaseRunning triggerPhase = iota
	phaseCompensating
)

type triggerRegistration struct {
	start     bool
	phase     triggerPhase
	correlate func(event.Event) (uuid.UUID, bool)
	handle    func(handlerRuntime) error
}

// AggregateID correlates using the aggregate ID of the triggering event.
func AggregateID[Data any]() Correlator[Data] {
	return func(evt event.Of[Data]) (uuid.UUID, bool) {
		id, _, _ := evt.Aggregate()
		return id, id != uuid.Nil
	}
}

// Define creates a saga definition from a typed constructor and trigger registrations.
func Define[S Instance](new func(uuid.UUID) S, regs ...Registration[S]) Definition {
	if new == nil {
		panic("saga.Define: nil constructor")
	}

	probe := new(internal.NewUUID())
	base := probe.SagaBase()
	if base == nil {
		panic("saga.Define: constructor returned saga with nil base")
	}

	def := &definition{
		name: base.AggregateName(),
		new: func(id uuid.UUID) Instance {
			return new(id)
		},
		registrations: make(map[string][]triggerRegistration),
	}

	if def.name == "" {
		panic("saga.Define: empty saga aggregate name")
	}

	for _, reg := range regs {
		if reg.apply != nil {
			reg.apply(def)
		}
	}

	return Definition{def: def}
}

// Starts registers a saga start handler.
func Starts[S Instance, Data any](
	correlate Correlator[Data],
	handler func(S, Ctx[Data]) error,
	eventNames ...string,
) Registration[S] {
	return register(phaseRunning, true, correlate, handler, eventNames...)
}

// Reacts registers a saga reaction handler.
func Reacts[S Instance, Data any](
	correlate Correlator[Data],
	handler func(S, Ctx[Data]) error,
	eventNames ...string,
) Registration[S] {
	return register(phaseRunning, false, correlate, handler, eventNames...)
}

// Compensates registers a compensation-phase reaction handler.
func Compensates[S Instance, Data any](
	correlate Correlator[Data],
	handler func(S, Ctx[Data]) error,
	eventNames ...string,
) Registration[S] {
	return register(phaseCompensating, false, correlate, handler, eventNames...)
}

// OnTimeout registers a timeout handler for the provided timeout key.
func OnTimeout[S Instance](
	key string,
	handler func(S, Ctx[TimeoutFiredData]) error,
) Registration[S] {
	key = strings.TrimSpace(key)
	if key == "" {
		panic("saga.OnTimeout: empty timeout key")
	}

	return Reacts(timeoutCorrelator(key), handler, TimeoutFired)
}

// OnCompensationTimeout registers a compensation-phase timeout handler for the provided timeout key.
func OnCompensationTimeout[S Instance](
	key string,
	handler func(S, Ctx[TimeoutFiredData]) error,
) Registration[S] {
	key = strings.TrimSpace(key)
	if key == "" {
		panic("saga.OnCompensationTimeout: empty timeout key")
	}

	return Compensates(timeoutCorrelator(key), handler, TimeoutFired)
}

func register[S Instance, Data any](
	phase triggerPhase,
	start bool,
	correlate Correlator[Data],
	handler func(S, Ctx[Data]) error,
	eventNames ...string,
) Registration[S] {
	if correlate == nil {
		panic("saga: nil correlator")
	}
	if handler == nil {
		panic("saga: nil handler")
	}
	if len(eventNames) == 0 {
		if start {
			panic("saga.Starts: no event names provided")
		}
		panic("saga.Reacts: no event names provided")
	}

	return Registration[S]{apply: func(def *definition) {
		for _, eventName := range eventNames {
			name := eventName
			def.registrations[name] = append(def.registrations[name], triggerRegistration{
				start: start,
				phase: phase,
				correlate: func(evt event.Event) (uuid.UUID, bool) {
					return correlate(castTrigger[Data](def.name, name, evt))
				},
				handle: func(rt handlerRuntime) error {
					inst, ok := rt.saga.(S)
					if !ok {
						var zero S
						panic(fmt.Errorf(
							"[goes/saga] cannot cast %T to %T [event=%s, saga=%s]",
							rt.saga, zero, name, def.name,
						))
					}
					casted := castTrigger[Data](def.name, name, rt.trigger)
					return handler(inst, newHandlerContext(rt, casted))
				},
			})
		}
	}}
}

func timeoutCorrelator(key string) Correlator[TimeoutFiredData] {
	return func(evt event.Of[TimeoutFiredData]) (uuid.UUID, bool) {
		data := evt.Data()
		if data.Key != key || data.SagaID == uuid.Nil {
			return uuid.Nil, false
		}
		return data.SagaID, true
	}
}

func castTrigger[Data any](sagaName, eventName string, evt event.Event) event.Of[Data] {
	casted, ok := event.TryCast[Data](evt)
	if ok {
		return casted
	}

	var zero Data
	panic(fmt.Errorf(
		"[goes/saga] cannot cast %T to %T; wrong trigger registration? [event=%s, saga=%s]",
		evt.Data(),
		zero,
		eventName,
		sagaName,
	))
}
