package saga

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

const (
	// Started marks the creation of a saga instance.
	Started = "goes.saga.started"

	// Completed marks a successfully finished saga instance.
	Completed = "goes.saga.completed"

	// Failed marks a failed saga instance.
	Failed = "goes.saga.failed"

	// CompensationStarted marks a saga entering compensation mode.
	CompensationStarted = "goes.saga.compensation.started"

	// CompensationCompleted marks a successfully compensated saga.
	CompensationCompleted = "goes.saga.compensation.completed"

	// CompensationFailed marks a compensation flow that ended unsuccessfully.
	CompensationFailed = "goes.saga.compensation.failed"

	// TriggerRecorded stores the external trigger IDs already processed by a saga.
	TriggerRecorded = "goes.saga.trigger.recorded"

	// CommandRequested records an outgoing command effect.
	CommandRequested = "goes.saga.command.requested"

	// CommandDispatched marks an outgoing command effect as dispatched.
	CommandDispatched = "goes.saga.command.dispatched"

	// TimeoutRequested records an active timeout.
	TimeoutRequested = "goes.saga.timeout.requested"

	// TimeoutCanceled cancels an active timeout.
	TimeoutCanceled = "goes.saga.timeout.canceled"

	// TimeoutFired records a fired timeout and can trigger saga reactions.
	TimeoutFired = "goes.saga.timeout.fired"
)

// Status describes the lifecycle of a saga instance.
type Status string

const (
	StatusRunning      Status = "running"
	StatusCompleted    Status = "completed"
	StatusFailed       Status = "failed"
	StatusCompensating Status = "compensating"
)

// Compensation describes persisted compensation metadata of a saga instance.
type Compensation struct {
	Active    bool
	Completed bool
	Failed    bool
	Error     string
}

// StartedData is the payload of Started.
type StartedData struct{}

// CompletedData is the payload of Completed.
type CompletedData struct{}

// FailedData is the payload of Failed.
type FailedData struct {
	Error string
}

// CompensationStartedData is the payload of CompensationStarted.
type CompensationStartedData struct {
	Reason string
}

// CompensationCompletedData is the payload of CompensationCompleted.
type CompensationCompletedData struct{}

// CompensationFailedData is the payload of CompensationFailed.
type CompensationFailedData struct {
	Error string
}

// TriggerRecordedData is the payload of TriggerRecorded.
type TriggerRecordedData struct {
	TriggerID   uuid.UUID
	TriggerName string
}

// CommandRequestedData is the payload of CommandRequested.
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

// CommandDispatchedData is the payload of CommandDispatched.
type CommandDispatchedData struct {
	EffectID uuid.UUID
}

// TimeoutRequestedData is the payload of TimeoutRequested.
type TimeoutRequestedData struct {
	EffectID  uuid.UUID
	Key       string
	TriggerID uuid.UUID
	At        time.Time
}

// TimeoutCanceledData is the payload of TimeoutCanceled.
type TimeoutCanceledData struct {
	EffectID uuid.UUID
	Key      string
}

// TimeoutFiredData is the payload of TimeoutFired.
type TimeoutFiredData struct {
	EffectID     uuid.UUID
	SagaID       uuid.UUID
	Key          string
	ScheduledFor time.Time
}

// RegisterEvents registers the built-in saga event types.
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

func (b *Base) recordFailed(err error) {
	data := FailedData{}
	if err != nil {
		data.Error = err.Error()
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

func (b *Base) recordCompensationFailed(err error) {
	data := CompensationFailedData{}
	if err != nil {
		data.Error = err.Error()
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
	b.failureReason = ""
	b.compensation = Compensation{}
}

func (b *Base) applyCompleted(event.Of[CompletedData]) {
	b.status = StatusCompleted
	b.failureReason = ""
	b.compensation = Compensation{}
}

func (b *Base) applyFailed(evt event.Of[FailedData]) {
	b.status = StatusFailed
	b.failureReason = evt.Data().Error
	b.compensation = Compensation{}
}

func (b *Base) applyCompensationStarted(evt event.Of[CompensationStartedData]) {
	b.status = StatusCompensating
	b.failureReason = evt.Data().Reason
	b.compensation = Compensation{Active: true}
}

func (b *Base) applyCompensationCompleted(event.Of[CompensationCompletedData]) {
	b.status = StatusFailed
	b.compensation = Compensation{Completed: true}
}

func (b *Base) applyCompensationFailed(evt event.Of[CompensationFailedData]) {
	b.status = StatusFailed
	b.compensation = Compensation{
		Failed: true,
		Error:  evt.Data().Error,
	}
}

func (b *Base) applyTriggerRecorded(evt event.Of[TriggerRecordedData]) {
	data := evt.Data()
	if data.TriggerID != uuid.Nil {
		b.triggers[data.TriggerID] = struct{}{}
	}
}

func (b *Base) applyCommandRequested(evt event.Of[CommandRequestedData]) {
	data := evt.Data()
	if data.EffectID == uuid.Nil {
		return
	}
	b.pendingCommands[data.EffectID] = data
}

func (b *Base) applyCommandDispatched(evt event.Of[CommandDispatchedData]) {
	delete(b.pendingCommands, evt.Data().EffectID)
}

func (b *Base) applyTimeoutRequested(evt event.Of[TimeoutRequestedData]) {
	data := evt.Data()
	if data.Key == "" || data.EffectID == uuid.Nil {
		return
	}

	key := timeoutMapKey(b.AggregateID(), data.Key)
	if current, ok := b.activeTimeouts[key]; ok && current.EffectID != data.EffectID {
		delete(b.timeoutByID, current.EffectID)
	}

	b.activeTimeouts[key] = data
	b.timeoutByID[data.EffectID] = data
}

func (b *Base) applyTimeoutCanceled(evt event.Of[TimeoutCanceledData]) {
	data := evt.Data()
	key := timeoutMapKey(b.AggregateID(), data.Key)
	if current, ok := b.activeTimeouts[key]; ok && current.EffectID == data.EffectID {
		delete(b.activeTimeouts, key)
	}
	delete(b.timeoutByID, data.EffectID)
}

func (b *Base) applyTimeoutFired(evt event.Of[TimeoutFiredData]) {
	data := evt.Data()
	key := timeoutMapKey(b.AggregateID(), data.Key)
	if current, ok := b.activeTimeouts[key]; ok && current.EffectID == data.EffectID {
		delete(b.activeTimeouts, key)
	}
	delete(b.timeoutByID, data.EffectID)
}

func decodeTimeoutFired(evt event.Event) (TimeoutFiredData, error) {
	fired, ok := event.TryCast[TimeoutFiredData](evt)
	if !ok {
		return TimeoutFiredData{}, fmt.Errorf("cast %q event to saga.TimeoutFiredData", evt.Name())
	}
	return fired.Data(), nil
}
