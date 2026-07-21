package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal"
)

// Definition is the static definition of a workflow type: its constructor,
// the trigger events that start and advance its instances, and optionally
// its own repository (see WithRepository). Definitions are created with
// Define and passed to NewService.
type Definition struct {
	def *definition
}

// An Option configures a workflow definition. Use Starts, Reacts,
// Compensates, OnTimeout, and OnCompensationTimeout to register trigger
// handlers, and WithRepository to configure the repository of the
// definition.
type Option[W Workflow] struct {
	apply func(*definition)
}

type definition struct {
	name          string
	create        func(uuid.UUID) Workflow
	registrations map[string][]triggerRegistration
	repo          *workflowRepository
}

// workflowRepository is the type-erased repository of a workflow definition.
type workflowRepository struct {
	fetch func(context.Context, uuid.UUID) (Workflow, error)
	save  func(context.Context, Workflow) error
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

// Define creates a workflow definition from a constructor and options:
//
//	def := workflow.Define(
//		NewOrderWorkflow,
//		workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onPlaced, OrderPlaced),
//		workflow.Reacts(workflow.ByAggregateID, (*OrderWorkflow).onPayment, PaymentReceived),
//		workflow.OnTimeout("payment", (*OrderWorkflow).onPaymentDue),
//
//		// Optional: fetch and save instances through a custom (e.g.
//		// snapshot-enabled) repository instead of the Service default.
//		// See WithRepository for details.
//		workflow.WithRepository(repository.Typed(repo, NewOrderWorkflow)),
//	)
func Define[W Workflow](create func(uuid.UUID) W, opts ...Option[W]) Definition {
	if create == nil {
		panic("workflow.Define: nil constructor")
	}

	probe := create(internal.NewUUID())
	base := probe.workflowBase()
	if base == nil {
		panic("workflow.Define: constructor returned workflow with nil base")
	}

	def := &definition{
		name: base.AggregateName(),
		create: func(id uuid.UUID) Workflow {
			return create(id)
		},
		registrations: make(map[string][]triggerRegistration),
	}

	if def.name == "" {
		panic("workflow.Define: empty workflow aggregate name")
	}

	for _, opt := range opts {
		if opt.apply != nil {
			opt.apply(def)
		}
	}

	return Definition{def: def}
}

// WithRepository configures the repository through which the instances of
// the workflow definition are fetched and saved, replacing the default
// repository of the Service (see Config.NewRepository). Use it to enable
// snapshots or repository hooks for a workflow type:
//
//	repo := repository.New(store, repository.WithSnapshots(snapshots, snapshot.Every(10)))
//
//	def := workflow.Define(
//		NewOrderWorkflow,
//		workflow.WithRepository(repository.Typed(repo, NewOrderWorkflow)),
//		workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onPlaced, OrderPlaced),
//	)
//
// The repository must persist to the same event store the Service is
// configured with: effect recovery and trigger replay read from
// Config.EventStore directly.
func WithRepository[W Workflow](repo aggregate.TypedRepository[W]) Option[W] {
	if repo == nil {
		panic("workflow.WithRepository: nil repository")
	}

	return Option[W]{apply: func(def *definition) {
		def.repo = &workflowRepository{
			fetch: func(ctx context.Context, id uuid.UUID) (Workflow, error) {
				return repo.Fetch(ctx, id)
			},
			save: func(ctx context.Context, w Workflow) error {
				return repo.Save(ctx, w.(W))
			},
		}
	}}
}

// Starts registers a handler for trigger events that may start a new
// workflow instance. If the correlated workflow already exists, the handler
// behaves like a Reacts handler.
func Starts[W Workflow, Data any](
	correlate Correlator[Data],
	handler func(W, Ctx[Data]) error,
	eventNames ...string,
) Option[W] {
	return register(phaseRunning, true, correlate, handler, eventNames...)
}

// Reacts registers a handler for trigger events that advance a running
// workflow. Events that correlate to an unknown workflow are ignored, unless
// the Service runs in strict mode.
func Reacts[W Workflow, Data any](
	correlate Correlator[Data],
	handler func(W, Ctx[Data]) error,
	eventNames ...string,
) Option[W] {
	return register(phaseRunning, false, correlate, handler, eventNames...)
}

// Compensates registers a handler for trigger events that advance a
// compensating workflow. Compensates handlers only run while the workflow
// has StatusCompensating.
func Compensates[W Workflow, Data any](
	correlate Correlator[Data],
	handler func(W, Ctx[Data]) error,
	eventNames ...string,
) Option[W] {
	return register(phaseCompensating, false, correlate, handler, eventNames...)
}

// OnTimeout registers a handler for the timeout with the given key,
// scheduled via Ctx.Schedule. The handler runs when the timeout fires while
// the workflow is running.
func OnTimeout[W Workflow](
	key string,
	handler func(W, Ctx[TimeoutFiredData]) error,
) Option[W] {
	key = strings.TrimSpace(key)
	if key == "" {
		panic("workflow.OnTimeout: empty timeout key")
	}

	return Reacts(timeoutCorrelator(key), handler, TimeoutFired)
}

// OnCompensationTimeout registers a handler for the timeout with the given
// key that runs when the timeout fires while the workflow is compensating.
func OnCompensationTimeout[W Workflow](
	key string,
	handler func(W, Ctx[TimeoutFiredData]) error,
) Option[W] {
	key = strings.TrimSpace(key)
	if key == "" {
		panic("workflow.OnCompensationTimeout: empty timeout key")
	}

	return Compensates(timeoutCorrelator(key), handler, TimeoutFired)
}

func register[W Workflow, Data any](
	phase triggerPhase,
	start bool,
	correlate Correlator[Data],
	handler func(W, Ctx[Data]) error,
	eventNames ...string,
) Option[W] {
	if correlate == nil {
		panic("workflow: nil correlator")
	}
	if handler == nil {
		panic("workflow: nil handler")
	}
	if len(eventNames) == 0 {
		if start {
			panic("workflow.Starts: no event names provided")
		}
		panic("workflow.Reacts: no event names provided")
	}

	return Option[W]{apply: func(def *definition) {
		for _, eventName := range eventNames {
			name := eventName
			def.registrations[name] = append(def.registrations[name], triggerRegistration{
				start: start,
				phase: phase,
				correlate: func(evt event.Event) (uuid.UUID, bool) {
					return correlate(castTrigger[Data](def.name, name, evt))
				},
				handle: func(rt handlerRuntime) error {
					w, ok := rt.workflow.(W)
					if !ok {
						var zero W
						panic(fmt.Errorf(
							"[goes/workflow] cannot cast %T to %T [event=%s, workflow=%s]",
							rt.workflow, zero, name, def.name,
						))
					}
					trigger := castTrigger[Data](def.name, name, rt.trigger)
					return handler(w, newHandlerContext(rt, trigger))
				},
			})
		}
	}}
}

func timeoutCorrelator(key string) Correlator[TimeoutFiredData] {
	return func(evt event.Of[TimeoutFiredData]) (uuid.UUID, bool) {
		data := evt.Data()
		if data.Key != key || data.WorkflowID == uuid.Nil {
			return uuid.Nil, false
		}
		return data.WorkflowID, true
	}
}

func castTrigger[Data any](workflowName, eventName string, evt event.Event) event.Of[Data] {
	casted, ok := event.TryCast[Data](evt)
	if ok {
		return casted
	}

	var zero Data
	panic(fmt.Errorf(
		"[goes/workflow] cannot cast %T to %T; wrong trigger registration? [event=%s, workflow=%s]",
		evt.Data(),
		zero,
		eventName,
		workflowName,
	))
}
