package workflow

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

// Built-in workflow events. These events are recorded on workflow instances
// by the runtime and by handler contexts, and drive the lifecycle and effect
// state of every workflow.
const (
	// Started marks the creation of a workflow instance.
	Started = "goes.workflow.started"

	// Completed marks a successfully finished workflow.
	Completed = "goes.workflow.completed"

	// Failed marks a failed workflow.
	Failed = "goes.workflow.failed"

	// CompensationStarted marks a workflow entering compensation.
	CompensationStarted = "goes.workflow.compensation.started"

	// CompensationCompleted marks a successfully compensated workflow.
	CompensationCompleted = "goes.workflow.compensation.completed"

	// CompensationFailed marks a compensation that ended unsuccessfully.
	CompensationFailed = "goes.workflow.compensation.failed"

	// TriggerRecorded stores the ids of trigger events already handled by a
	// workflow, making trigger handling idempotent.
	TriggerRecorded = "goes.workflow.trigger.recorded"

	// CommandRequested records an outgoing command effect.
	CommandRequested = "goes.workflow.command.requested"

	// CommandDispatched marks an outgoing command effect as dispatched.
	CommandDispatched = "goes.workflow.command.dispatched"

	// TimeoutRequested records an active timeout.
	TimeoutRequested = "goes.workflow.timeout.requested"

	// TimeoutCanceled cancels an active timeout.
	TimeoutCanceled = "goes.workflow.timeout.canceled"

	// TimeoutFired records a fired timeout and triggers timeout handlers.
	TimeoutFired = "goes.workflow.timeout.fired"
)

// StartedData is the payload of the Started event.
type StartedData struct{}

// CompletedData is the payload of the Completed event.
type CompletedData struct{}

// FailedData is the payload of the Failed event.
type FailedData struct {
	Reason string
}

// CompensationStartedData is the payload of the CompensationStarted event.
type CompensationStartedData struct {
	Reason string
}

// CompensationCompletedData is the payload of the CompensationCompleted event.
type CompensationCompletedData struct{}

// CompensationFailedData is the payload of the CompensationFailed event.
type CompensationFailedData struct {
	Reason string
}

// TriggerRecordedData is the payload of the TriggerRecorded event.
type TriggerRecordedData struct {
	TriggerID   uuid.UUID
	TriggerName string
}

// CommandRequestedData is the payload of the CommandRequested event.
type CommandRequestedData struct {
	EffectID      uuid.UUID
	Key           string
	TriggerID     uuid.UUID
	CommandID     uuid.UUID
	Name          string
	AggregateName string
	AggregateID   uuid.UUID
	Payload       []byte
}

// CommandDispatchedData is the payload of the CommandDispatched event.
type CommandDispatchedData struct {
	EffectID uuid.UUID
}

// TimeoutRequestedData is the payload of the TimeoutRequested event.
type TimeoutRequestedData struct {
	EffectID  uuid.UUID
	Key       string
	TriggerID uuid.UUID
	At        time.Time
}

// TimeoutCanceledData is the payload of the TimeoutCanceled event.
type TimeoutCanceledData struct {
	EffectID uuid.UUID
	Key      string
}

// TimeoutFiredData is the payload of the TimeoutFired event.
type TimeoutFiredData struct {
	EffectID     uuid.UUID
	WorkflowID   uuid.UUID
	Key          string
	ScheduledFor time.Time
}

// RegisterEvents registers the built-in workflow events into a codec
// registry, so that codec-backed event stores and buses can encode and
// decode their payloads.
func RegisterEvents(r codec.Registerer) {
	codec.Register[StartedData](r, Started)
	codec.Register[CompletedData](r, Completed)
	codec.Register[FailedData](r, Failed)
	codec.Register[CompensationStartedData](r, CompensationStarted)
	codec.Register[CompensationCompletedData](r, CompensationCompleted)
	codec.Register[CompensationFailedData](r, CompensationFailed)
	codec.Register[TriggerRecordedData](r, TriggerRecorded)
	codec.Register[CommandRequestedData](r, CommandRequested)
	codec.Register[CommandDispatchedData](r, CommandDispatched)
	codec.Register[TimeoutRequestedData](r, TimeoutRequested)
	codec.Register[TimeoutCanceledData](r, TimeoutCanceled)
	codec.Register[TimeoutFiredData](r, TimeoutFired)
}

func (b *Base) recordStarted() {
	aggregate.Next(b, Started, StartedData{})
}

func (b *Base) recordCompleted() {
	aggregate.Next(b, Completed, CompletedData{})
}

func (b *Base) recordFailed(reason error) {
	data := FailedData{}
	if reason != nil {
		data.Reason = reason.Error()
	}
	aggregate.Next(b, Failed, data)
}

func (b *Base) recordCompensationStarted(reason error) {
	data := CompensationStartedData{}
	if reason != nil {
		data.Reason = reason.Error()
	}
	aggregate.Next(b, CompensationStarted, data)
}

func (b *Base) recordCompensationCompleted() {
	aggregate.Next(b, CompensationCompleted, CompensationCompletedData{})
}

func (b *Base) recordCompensationFailed(reason error) {
	data := CompensationFailedData{}
	if reason != nil {
		data.Reason = reason.Error()
	}
	aggregate.Next(b, CompensationFailed, data)
}

func (b *Base) recordTrigger(evt event.Event) {
	aggregate.Next(b, TriggerRecorded, TriggerRecordedData{
		TriggerID:   evt.ID(),
		TriggerName: evt.Name(),
	})
}

func (b *Base) recordCommandRequested(data CommandRequestedData) {
	aggregate.Next(b, CommandRequested, data, event.ID(data.EffectID))
}

func (b *Base) recordCommandDispatched(effectID uuid.UUID) {
	aggregate.Next(b, CommandDispatched, CommandDispatchedData{
		EffectID: effectID,
	})
}

func (b *Base) recordTimeoutRequested(data TimeoutRequestedData) {
	aggregate.Next(b, TimeoutRequested, data, event.ID(data.EffectID))
}

func (b *Base) recordTimeoutCanceled(data TimeoutCanceledData) {
	aggregate.Next(b, TimeoutCanceled, data)
}

func (b *Base) recordTimeoutFired(data TimeoutFiredData, id uuid.UUID) {
	aggregate.Next(b, TimeoutFired, data, event.ID(id))
}

func (b *Base) applyStarted(event.Of[StartedData]) {
	b.status = StatusRunning
	b.reason = ""
}

func (b *Base) applyCompleted(event.Of[CompletedData]) {
	b.status = StatusCompleted
	b.reason = ""
}

func (b *Base) applyFailed(evt event.Of[FailedData]) {
	b.status = StatusFailed
	b.reason = evt.Data().Reason
}

func (b *Base) applyCompensationStarted(evt event.Of[CompensationStartedData]) {
	b.status = StatusCompensating
	b.reason = evt.Data().Reason
}

func (b *Base) applyCompensationCompleted(event.Of[CompensationCompletedData]) {
	b.status = StatusCompensated
}

func (b *Base) applyCompensationFailed(evt event.Of[CompensationFailedData]) {
	b.status = StatusFailed
	b.reason = evt.Data().Reason
}

func (b *Base) applyTriggerRecorded(evt event.Of[TriggerRecordedData]) {
	if id := evt.Data().TriggerID; id != uuid.Nil {
		b.triggers[id] = struct{}{}
	}
}

func (b *Base) applyCommandRequested(evt event.Of[CommandRequestedData]) {
	data := evt.Data()
	if data.EffectID == uuid.Nil {
		return
	}
	b.commands[data.EffectID] = data
}

func (b *Base) applyCommandDispatched(evt event.Of[CommandDispatchedData]) {
	delete(b.commands, evt.Data().EffectID)
}

func (b *Base) applyTimeoutRequested(evt event.Of[TimeoutRequestedData]) {
	data := evt.Data()
	if data.Key == "" || data.EffectID == uuid.Nil {
		return
	}

	if current, ok := b.timeoutsByKey[data.Key]; ok && current.EffectID != data.EffectID {
		delete(b.timeoutsByID, current.EffectID)
	}

	b.timeoutsByKey[data.Key] = data
	b.timeoutsByID[data.EffectID] = data
}

func (b *Base) applyTimeoutCanceled(evt event.Of[TimeoutCanceledData]) {
	data := evt.Data()
	if current, ok := b.timeoutsByKey[data.Key]; ok && current.EffectID == data.EffectID {
		delete(b.timeoutsByKey, data.Key)
	}
	delete(b.timeoutsByID, data.EffectID)
}

func (b *Base) applyTimeoutFired(evt event.Of[TimeoutFiredData]) {
	data := evt.Data()
	if current, ok := b.timeoutsByKey[data.Key]; ok && current.EffectID == data.EffectID {
		delete(b.timeoutsByKey, data.Key)
	}
	delete(b.timeoutsByID, data.EffectID)
}

func decodeTimeoutFired(evt event.Event) (TimeoutFiredData, error) {
	fired, ok := event.TryCast[TimeoutFiredData](evt)
	if !ok {
		return TimeoutFiredData{}, fmt.Errorf("cast %q event to workflow.TimeoutFiredData", evt.Name())
	}
	return fired.Data(), nil
}
