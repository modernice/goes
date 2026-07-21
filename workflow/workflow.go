// Package workflow provides an event-driven, distributed workflow runtime,
// also known as a saga or process manager.
//
// A workflow instance is an event-sourced aggregate. Domain events start and
// advance workflows, and handlers record durable effects — outgoing commands
// and timeouts — as events on the workflow instead of executing them inline.
// A background runtime executes recorded effects and recovers them from the
// event store after restarts, so a workflow never loses an effect to a crash.
//
// Workflows are defined statically with Define and run by a Service:
//
//	def := workflow.Define(
//		NewOrderWorkflow,
//		workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onPlaced, OrderPlaced),
//		workflow.Reacts(workflow.ByAggregateID, (*OrderWorkflow).onPayment, PaymentReceived),
//		workflow.OnTimeout("payment", (*OrderWorkflow).onPaymentTimeout),
//	)
//
//	svc := workflow.NewService(workflow.Config{
//		EventStore: store,
//		EventBus:   eventBus,
//		CommandBus: commandBus,
//		Commands:   registry,
//	}, def)
//
//	errs, err := svc.Run(ctx)
package workflow

import (
	"sort"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// Workflow is a single workflow instance. Workflows are event-sourced
// aggregates that embed *Base, which provides the lifecycle state of the
// workflow and satisfies this interface:
//
//	type OrderWorkflow struct {
//		*workflow.Base
//	}
//
//	func NewOrderWorkflow(id uuid.UUID) *OrderWorkflow {
//		return &OrderWorkflow{Base: workflow.New("shop.order_workflow", id)}
//	}
type Workflow interface {
	aggregate.TypedAggregate

	workflowBase() *Base
}

// Status is the lifecycle status of a workflow instance.
type Status string

const (
	// StatusRunning is the status of a started workflow that has not reached
	// a terminal status yet.
	StatusRunning Status = "running"

	// StatusCompleted is the terminal status of a successfully finished
	// workflow.
	StatusCompleted Status = "completed"

	// StatusCompensating is the status of a workflow that is undoing its
	// previous work after a business failure.
	StatusCompensating Status = "compensating"

	// StatusCompensated is the terminal status of a workflow whose
	// compensation finished successfully.
	StatusCompensated Status = "compensated"

	// StatusFailed is the terminal status of a workflow that failed, either
	// directly or because its compensation failed.
	StatusFailed Status = "failed"
)

// Base is the embeddable base type for workflow instances. It maintains the
// lifecycle status of the workflow and the state of its recorded effects,
// all derived from the built-in workflow events.
type Base struct {
	*aggregate.Base

	status Status
	reason string

	triggers      map[uuid.UUID]struct{}
	commands      map[uuid.UUID]CommandRequestedData
	timeoutsByKey map[string]TimeoutRequestedData
	timeoutsByID  map[uuid.UUID]TimeoutRequestedData
}

// New returns a new workflow base for the workflow with the given aggregate
// name and id.
func New(name string, id uuid.UUID) *Base {
	b := &Base{
		Base:          aggregate.New(name, id),
		triggers:      make(map[uuid.UUID]struct{}),
		commands:      make(map[uuid.UUID]CommandRequestedData),
		timeoutsByKey: make(map[string]TimeoutRequestedData),
		timeoutsByID:  make(map[uuid.UUID]TimeoutRequestedData),
	}

	event.ApplyWith(b, b.applyStarted, Started)
	event.ApplyWith(b, b.applyCompleted, Completed)
	event.ApplyWith(b, b.applyFailed, Failed)
	event.ApplyWith(b, b.applyCompensationStarted, CompensationStarted)
	event.ApplyWith(b, b.applyCompensationCompleted, CompensationCompleted)
	event.ApplyWith(b, b.applyCompensationFailed, CompensationFailed)
	event.ApplyWith(b, b.applyTriggerRecorded, TriggerRecorded)
	event.ApplyWith(b, b.applyCommandRequested, CommandRequested)
	event.ApplyWith(b, b.applyCommandDispatched, CommandDispatched)
	event.ApplyWith(b, b.applyTimeoutRequested, TimeoutRequested)
	event.ApplyWith(b, b.applyTimeoutCanceled, TimeoutCanceled)
	event.ApplyWith(b, b.applyTimeoutFired, TimeoutFired)

	return b
}

// workflowBase makes types that embed *Base satisfy the Workflow interface.
func (b *Base) workflowBase() *Base { return b }

// Status returns the lifecycle status of the workflow.
func (b *Base) Status() Status { return b.status }

// Reason returns why the workflow left the happy path: the reason of the
// most recent Fail, Compensate, or CompensationFailed transition. It returns
// an empty string for running and completed workflows.
func (b *Base) Reason() string { return b.reason }

// Done reports whether the workflow reached a terminal status
// (StatusCompleted, StatusCompensated, or StatusFailed).
func (b *Base) Done() bool {
	switch b.status {
	case StatusCompleted, StatusCompensated, StatusFailed:
		return true
	}
	return false
}

func (b *Base) compensating() bool {
	return b.status == StatusCompensating
}

func (b *Base) handledTrigger(id uuid.UUID) bool {
	_, ok := b.triggers[id]
	return ok
}

func (b *Base) pendingCommand(id uuid.UUID) (CommandRequestedData, bool) {
	data, ok := b.commands[id]
	return data, ok
}

func (b *Base) activeTimeout(key string) (TimeoutRequestedData, bool) {
	data, ok := b.timeoutsByKey[key]
	return data, ok
}

func (b *Base) activeTimeoutByID(id uuid.UUID) (TimeoutRequestedData, bool) {
	data, ok := b.timeoutsByID[id]
	return data, ok
}

// cancelTimeouts cancels all active timeouts of the workflow, in
// deterministic (sorted) order.
func (b *Base) cancelTimeouts() {
	keys := make([]string, 0, len(b.timeoutsByKey))
	for key := range b.timeoutsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		timeout, ok := b.timeoutsByKey[key]
		if !ok {
			continue
		}

		b.recordTimeoutCanceled(TimeoutCanceledData{
			EffectID: timeout.EffectID,
			Key:      timeout.Key,
		})
	}
}
