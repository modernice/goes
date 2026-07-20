package saga

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

// Base is the embeddable base type for event-driven saga instances.
type Base struct {
	*aggregate.Base

	status          Status
	failureReason   string
	compensation    Compensation
	triggers        map[uuid.UUID]struct{}
	pendingCommands map[uuid.UUID]CommandRequestedData
	activeTimeouts  map[string]TimeoutRequestedData
	timeoutByID     map[uuid.UUID]TimeoutRequestedData
}

// New returns a new saga base.
func New(name string, id uuid.UUID) *Base {
	b := &Base{
		Base:            aggregate.New(name, id),
		triggers:        make(map[uuid.UUID]struct{}),
		pendingCommands: make(map[uuid.UUID]CommandRequestedData),
		activeTimeouts:  make(map[string]TimeoutRequestedData),
		timeoutByID:     make(map[uuid.UUID]TimeoutRequestedData),
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

// SagaBase returns b. Embedding *Base makes a saga satisfy saga.Registerer and saga.Instance.
func (b *Base) SagaBase() *Base {
	return b
}

// Status returns the current lifecycle status of the saga instance.
func (b *Base) Status() Status {
	return b.status
}

// FailureReason returns the persisted failure reason of the saga.
func (b *Base) FailureReason() string {
	return b.failureReason
}

// Compensation returns the persisted compensation metadata of the saga.
func (b *Base) Compensation() Compensation {
	return b.compensation
}

func (b *Base) terminal() bool {
	return b.status == StatusCompleted || b.status == StatusFailed
}

func (b *Base) compensating() bool {
	return b.status == StatusCompensating
}

func (b *Base) handledTrigger(id uuid.UUID) bool {
	_, ok := b.triggers[id]
	return ok
}

func (b *Base) hasPendingCommand(id uuid.UUID) bool {
	_, ok := b.pendingCommands[id]
	return ok
}

func (b *Base) activeTimeout(key string) (TimeoutRequestedData, bool) {
	data, ok := b.activeTimeouts[timeoutMapKey(b.AggregateID(), key)]
	return data, ok
}

func (b *Base) cancelActiveTimeouts() {
	keys := make([]string, 0, len(b.activeTimeouts))
	for key := range b.activeTimeouts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		timeout, ok := b.activeTimeouts[key]
		if !ok {
			continue
		}

		b.recordTimeoutCanceled(TimeoutCanceledData{
			EffectID: timeout.EffectID,
			Key:      timeout.Key,
		})
	}
}

type handlerRuntime struct {
	context.Context
	saga    Instance
	base    *Base
	trigger event.Event
	enc     codec.Encoding
}
