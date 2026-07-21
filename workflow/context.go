package workflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
)

var (
	// ErrEmptyEffectKey is returned when an effect key is empty.
	ErrEmptyEffectKey = errors.New("empty effect key")

	// ErrEffectConflict is returned when a handler records an effect under a
	// key that is already recorded with a different command.
	ErrEffectConflict = errors.New("conflicting effect")

	// ErrInvalidTransition is returned when a lifecycle transition is not
	// allowed in the current status of the workflow.
	ErrInvalidTransition = errors.New("invalid workflow transition")

	// ErrNotCompensating is returned when a compensation-only transition is
	// used outside of compensation.
	ErrNotCompensating = errors.New("workflow is not compensating")
)

// Ctx is the context passed to workflow trigger handlers. It gives handlers
// access to the trigger event and records effects and lifecycle transitions
// as events on the workflow. Effects are not executed inline: they are
// persisted together with the other changes of the workflow and executed by
// the Service runtime.
type Ctx[Data any] interface {
	context.Context

	// Event returns the trigger event.
	Event() event.Of[Data]

	// Dispatch records an outgoing command effect under the given key. The
	// runtime dispatches the command after the workflow was saved, and
	// re-dispatches it after restarts until the dispatch was recorded, making
	// command dispatch at-least-once. Recording the same key with the same
	// command twice is a no-op; recording it with a different command returns
	// ErrEffectConflict.
	Dispatch(key string, cmd command.Command) error

	// Schedule records a timeout with the given key that fires at the given
	// time. Scheduling a key that already has an active timeout replaces it.
	Schedule(key string, at time.Time) error

	// Unschedule cancels the active timeout with the given key. Canceling
	// a key without an active timeout is a no-op.
	Unschedule(key string) error

	// Complete transitions the workflow to StatusCompleted and cancels all
	// active timeouts.
	Complete() error

	// Fail transitions the workflow to StatusFailed and cancels all active
	// timeouts.
	Fail(reason error) error

	// Compensate transitions the workflow to StatusCompensating and cancels
	// all active timeouts. From here on, only Compensates and
	// OnCompensationTimeout handlers run.
	Compensate(reason error) error

	// Compensated transitions the compensating workflow to
	// StatusCompensated.
	Compensated() error

	// CompensationFailed transitions the compensating workflow to
	// StatusFailed.
	CompensationFailed(reason error) error
}

// handlerRuntime carries the per-trigger state that a handler context needs.
type handlerRuntime struct {
	context.Context

	workflow Workflow
	base     *Base
	trigger  event.Event
	enc      codec.Encoding
}

type handlerContext[Data any] struct {
	handlerRuntime

	evt event.Of[Data]
}

func newHandlerContext[Data any](rt handlerRuntime, evt event.Of[Data]) Ctx[Data] {
	return &handlerContext[Data]{
		handlerRuntime: rt,
		evt:            evt,
	}
}

func (ctx *handlerContext[Data]) Event() event.Of[Data] {
	return ctx.evt
}

func (ctx *handlerContext[Data]) Dispatch(key string, cmd command.Command) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return ErrEmptyEffectKey
	}

	if ctx.enc == nil {
		return fmt.Errorf("encode %q command payload: no encoding", cmd.Name())
	}

	payload, err := ctx.enc.Marshal(cmd.Payload())
	if err != nil {
		return fmt.Errorf("encode %q command payload: %w", cmd.Name(), err)
	}

	id := effectID("command", ctx.base.AggregateID(), ctx.trigger.ID(), key)
	aggregateID, aggregateName := cmd.Aggregate().Split()

	if current, ok := ctx.base.pendingCommand(id); ok {
		if current.Name == cmd.Name() &&
			current.AggregateName == aggregateName &&
			current.AggregateID == aggregateID &&
			bytes.Equal(current.Payload, payload) {
			return nil
		}
		return fmt.Errorf("dispatch %q command as %q effect: %w", cmd.Name(), key, ErrEffectConflict)
	}

	ctx.base.recordCommandRequested(CommandRequestedData{
		EffectID:      id,
		Key:           key,
		TriggerID:     ctx.trigger.ID(),
		CommandID:     commandID(id),
		Name:          cmd.Name(),
		AggregateName: aggregateName,
		AggregateID:   aggregateID,
		Payload:       payload,
	})

	return nil
}

func (ctx *handlerContext[Data]) Schedule(key string, at time.Time) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return ErrEmptyEffectKey
	}

	id := effectID("timeout", ctx.base.AggregateID(), ctx.trigger.ID(), key)
	if current, ok := ctx.base.activeTimeout(key); ok {
		if current.EffectID == id && current.At.Equal(at) {
			return nil
		}

		ctx.base.recordTimeoutCanceled(TimeoutCanceledData{
			EffectID: current.EffectID,
			Key:      current.Key,
		})
	}

	ctx.base.recordTimeoutRequested(TimeoutRequestedData{
		EffectID:  id,
		Key:       key,
		TriggerID: ctx.trigger.ID(),
		At:        at,
	})

	return nil
}

func (ctx *handlerContext[Data]) Unschedule(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return ErrEmptyEffectKey
	}

	current, ok := ctx.base.activeTimeout(key)
	if !ok {
		return nil
	}

	ctx.base.recordTimeoutCanceled(TimeoutCanceledData{
		EffectID: current.EffectID,
		Key:      current.Key,
	})

	return nil
}

func (ctx *handlerContext[Data]) Complete() error {
	switch ctx.base.Status() {
	case StatusCompleted:
		return nil
	case StatusCompensating, StatusCompensated, StatusFailed:
		return ErrInvalidTransition
	}

	ctx.base.cancelTimeouts()
	ctx.base.recordCompleted()
	return nil
}

func (ctx *handlerContext[Data]) Fail(reason error) error {
	switch ctx.base.Status() {
	case StatusFailed:
		return nil
	case StatusCompleted, StatusCompensating, StatusCompensated:
		return ErrInvalidTransition
	}

	ctx.base.cancelTimeouts()
	ctx.base.recordFailed(reason)
	return nil
}

func (ctx *handlerContext[Data]) Compensate(reason error) error {
	switch ctx.base.Status() {
	case StatusCompensating:
		return nil
	case StatusCompleted, StatusCompensated, StatusFailed:
		return ErrInvalidTransition
	}

	ctx.base.cancelTimeouts()
	ctx.base.recordCompensationStarted(reason)
	return nil
}

func (ctx *handlerContext[Data]) Compensated() error {
	switch ctx.base.Status() {
	case StatusCompensated:
		return nil
	case StatusCompensating:
	default:
		return ErrNotCompensating
	}

	ctx.base.cancelTimeouts()
	ctx.base.recordCompensationCompleted()
	return nil
}

func (ctx *handlerContext[Data]) CompensationFailed(reason error) error {
	if !ctx.base.compensating() {
		return ErrNotCompensating
	}

	ctx.base.cancelTimeouts()
	ctx.base.recordCompensationFailed(reason)
	return nil
}
