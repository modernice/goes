package saga

import (
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

	// ErrInvalidTransition is returned when a lifecycle transition is not allowed.
	ErrInvalidTransition = errors.New("invalid saga transition")

	// ErrNotCompensating is returned when a compensation-only operation is used outside compensation.
	ErrNotCompensating = errors.New("saga is not compensating")
)

// Ctx is the handler context passed to saga trigger handlers.
type Ctx[D any] interface {
	context.Context

	// Event returns the triggering event.
	Event() event.Of[D]

	// Dispatch records an outgoing command effect.
	Dispatch(step string, cmd command.Command) error

	// Schedule records an active timeout.
	Schedule(timeout string, at time.Time) error

	// CancelTimeout cancels an active timeout by key.
	CancelTimeout(timeout string) error

	// Complete marks the saga as completed.
	Complete() error

	// Fail marks the saga as failed.
	Fail(error) error

	// Compensate enters compensation mode.
	Compensate(reason error) error

	// Compensated marks compensation as completed successfully.
	Compensated() error

	// CompensationFailed marks compensation as failed.
	CompensationFailed(error) error
}

type handlerContext[D any] struct {
	handlerRuntime
	evt event.Of[D]
	enc codec.Encoding
}

func newHandlerContext[D any](rt handlerRuntime, evt event.Of[D]) Ctx[D] {
	return &handlerContext[D]{
		handlerRuntime: rt,
		evt:            evt,
		enc:            rt.enc,
	}
}

func (ctx *handlerContext[D]) Event() event.Of[D] {
	return ctx.evt
}

func (ctx *handlerContext[D]) Dispatch(key string, cmd command.Command) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return ErrEmptyEffectKey
	}

	effectID := effectID("command", ctx.base.AggregateID(), ctx.trigger.ID(), key)
	if ctx.base.hasPendingCommand(effectID) {
		return nil
	}

	if ctx.enc == nil {
		return fmt.Errorf("encode command payload: missing codec encoding")
	}

	payload, err := ctx.enc.Marshal(cmd.Payload())
	if err != nil {
		return fmt.Errorf("encode command payload: %w", err)
	}

	aggregateID, aggregateName := cmd.Aggregate().Split()
	ctx.base.recordCommandRequested(CommandRequestedData{
		EffectID:      effectID,
		Key:           key,
		TriggerID:     ctx.trigger.ID(),
		CommandID:     commandID(effectID),
		Name:          cmd.Name(),
		AggregateName: aggregateName,
		AggregateID:   aggregateID,
		Payload:       payload,
	})

	return nil
}

func (ctx *handlerContext[D]) Schedule(key string, at time.Time) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return ErrEmptyEffectKey
	}

	effectID := effectID("timeout", ctx.base.AggregateID(), ctx.trigger.ID(), key)
	if current, ok := ctx.base.activeTimeout(key); ok {
		if current.EffectID == effectID && current.At.Equal(at) {
			return nil
		}

		ctx.base.recordTimeoutCanceled(TimeoutCanceledData{
			EffectID: current.EffectID,
			Key:      current.Key,
		})
	}

	ctx.base.recordTimeoutRequested(TimeoutRequestedData{
		EffectID:  effectID,
		Key:       key,
		TriggerID: ctx.trigger.ID(),
		At:        at,
	})

	return nil
}

func (ctx *handlerContext[D]) CancelTimeout(key string) error {
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

func (ctx *handlerContext[D]) Complete() error {
	switch ctx.base.Status() {
	case StatusCompleted:
		return nil
	case StatusCompensating, StatusFailed:
		return ErrInvalidTransition
	}

	ctx.base.cancelActiveTimeouts()
	ctx.base.recordCompleted()
	return nil
}

func (ctx *handlerContext[D]) Fail(err error) error {
	switch ctx.base.Status() {
	case StatusFailed:
		return nil
	case StatusCompleted, StatusCompensating:
		return ErrInvalidTransition
	}

	ctx.base.cancelActiveTimeouts()
	ctx.base.recordFailed(err)
	return nil
}

func (ctx *handlerContext[D]) Compensate(reason error) error {
	switch ctx.base.Status() {
	case StatusCompensating:
		return nil
	case StatusCompleted, StatusFailed:
		return ErrInvalidTransition
	}

	ctx.base.cancelActiveTimeouts()
	ctx.base.recordCompensationStarted(reason)
	return nil
}

func (ctx *handlerContext[D]) Compensated() error {
	if !ctx.base.compensating() {
		return ErrNotCompensating
	}

	ctx.base.cancelActiveTimeouts()
	ctx.base.recordCompensationCompleted()
	return nil
}

func (ctx *handlerContext[D]) CompensationFailed(err error) error {
	if !ctx.base.compensating() {
		return ErrNotCompensating
	}

	ctx.base.cancelActiveTimeouts()
	ctx.base.recordCompensationFailed(err)
	return nil
}
