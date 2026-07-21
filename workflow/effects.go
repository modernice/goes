package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	qtime "github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/helper/streams"
)

// resyncOverlap is how far a periodic resync reaches back before the
// previous sync, covering clock skew between service instances and events
// that were committed while the previous sync ran.
const resyncOverlap = time.Minute

// takeoverGrace is how long effects discovered from the event store are left
// to the instance that recorded them before another instance attempts them.
// The grace is anchored to the recording time of the effect, so effects that
// are already older than the grace — typically during crash recovery — are
// attempted without delay. Effects recorded by the local instance are
// notified directly and carry no grace.
const takeoverGrace = 500 * time.Millisecond

// pendingCommand references a pending command effect. The command data
// itself (payload, target aggregate) is not tracked in memory: it is read
// from the fetched workflow when the command is dispatched.
type pendingCommand struct {
	workflow  string
	id        uuid.UUID
	effectID  uuid.UUID
	notBefore time.Time
}

type activeTimeout struct {
	workflow  string
	id        uuid.UUID
	data      TimeoutRequestedData
	notBefore time.Time
}

type timeoutRef struct {
	id  uuid.UUID
	key string
}

// effectState is the in-memory view of the pending effects of all workflow
// instances. It is a hint, not the source of truth: before an effect is
// executed, its state is re-validated against the freshly fetched workflow,
// so stale or duplicated entries are harmless.
type effectState struct {
	commands map[uuid.UUID]pendingCommand
	timeouts map[timeoutRef]activeTimeout
}

func newEffectState() *effectState {
	return &effectState{
		commands: make(map[uuid.UUID]pendingCommand),
		timeouts: make(map[timeoutRef]activeTimeout),
	}
}

// apply folds an effect event into the state. notBefore delays the first
// execution attempt of newly discovered effects; effects that are already
// tracked keep their timing.
func (state *effectState) apply(evt event.Event, notBefore time.Time) error {
	id, name, _ := evt.Aggregate()

	switch evt.Name() {
	case CommandRequested:
		data, err := castEffect[CommandRequestedData](evt)
		if err != nil {
			return err
		}
		if _, ok := state.commands[data.EffectID]; ok {
			return nil
		}
		state.commands[data.EffectID] = pendingCommand{workflow: name, id: id, effectID: data.EffectID, notBefore: notBefore}
	case CommandDispatched:
		data, err := castEffect[CommandDispatchedData](evt)
		if err != nil {
			return err
		}
		delete(state.commands, data.EffectID)
	case TimeoutRequested:
		data, err := castEffect[TimeoutRequestedData](evt)
		if err != nil {
			return err
		}
		ref := timeoutRef{id, data.Key}
		if current, ok := state.timeouts[ref]; ok && current.data.EffectID == data.EffectID {
			return nil
		}
		state.timeouts[ref] = activeTimeout{workflow: name, id: id, data: data, notBefore: notBefore}
	case TimeoutCanceled:
		data, err := castEffect[TimeoutCanceledData](evt)
		if err != nil {
			return err
		}
		ref := timeoutRef{id, data.Key}
		if current, ok := state.timeouts[ref]; ok && current.data.EffectID == data.EffectID {
			delete(state.timeouts, ref)
		}
	case TimeoutFired:
		data, err := castEffect[TimeoutFiredData](evt)
		if err != nil {
			return err
		}
		ref := timeoutRef{id, data.Key}
		if current, ok := state.timeouts[ref]; ok && current.data.EffectID == data.EffectID {
			delete(state.timeouts, ref)
		}
	}

	return nil
}

func castEffect[Data any](evt event.Event) (Data, error) {
	casted, ok := event.TryCast[Data](evt)
	if !ok {
		var zero Data
		return zero, fmt.Errorf("cast %q event to %T", evt.Name(), zero)
	}
	return casted.Data(), nil
}

// runEffects runs the effect runtime: it recovers pending effects from the
// event store, keeps them up to date through save notifications and periodic
// resyncs, dispatches pending commands, and fires due timeouts.
func (s *Service) runEffects(ctx context.Context) {
	state := newEffectState()

	var lastSync time.Time
	if synced, err := s.resync(ctx, state, lastSync); err != nil {
		s.report(fmt.Errorf("recover workflow effects: %w", err))
	} else {
		lastSync = synced
	}

	commands := time.NewTicker(s.dispatchInterval)
	defer commands.Stop()

	timeouts := time.NewTicker(s.timerResolution)
	defer timeouts.Stop()

	var resync <-chan time.Time
	if s.resyncInterval > 0 {
		ticker := time.NewTicker(s.resyncInterval)
		defer ticker.Stop()
		resync = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-s.updates:
			if err := state.apply(evt, time.Time{}); err != nil {
				s.report(err)
			}
		case now := <-commands.C:
			s.dispatchDue(ctx, state, now)
		case now := <-timeouts.C:
			s.fireDue(ctx, state, now)
		case <-resync:
			if synced, err := s.resync(ctx, state, lastSync); err != nil {
				s.report(fmt.Errorf("resync workflow effects: %w", err))
			} else {
				lastSync = synced
			}
		}
	}
}

// resync folds the effect events of all registered workflow types from the
// event store into the effect state. The initial resync (zero since) reads
// the history bounded by the recovery window; periodic resyncs only read
// events since the previous sync, with an overlap to cover clock skew
// between instances.
func (s *Service) resync(ctx context.Context, state *effectState, since time.Time) (time.Time, error) {
	opts := []query.Option{
		query.Name(CommandRequested, CommandDispatched, TimeoutRequested, TimeoutCanceled, TimeoutFired),
		query.AggregateName(s.names...),
		query.SortBy(event.SortAggregateVersion, event.SortAsc),
	}
	if !since.IsZero() {
		opts = append(opts, query.Time(qtime.After(since.Add(-resyncOverlap))))
	} else if s.recoveryWindow > 0 {
		opts = append(opts, query.Time(qtime.After(time.Now().Add(-s.recoveryWindow))))
	}

	now := time.Now()
	events, errs, err := s.store.Query(ctx, query.New(opts...))
	if err != nil {
		return time.Time{}, fmt.Errorf("query effect events: %w", err)
	}

	if err := streams.Walk(ctx, func(evt event.Event) error {
		// Give the instance that recorded the effect a head start before
		// attempting it, so healthy instances rarely duplicate each other's
		// fresh effects.
		return state.apply(evt, evt.Time().Add(takeoverGrace))
	}, events, errs); err != nil {
		return time.Time{}, err
	}

	return now, nil
}

func (s *Service) dispatchDue(ctx context.Context, state *effectState, now time.Time) {
	for id, cmd := range state.commands {
		if cmd.notBefore.After(now) {
			continue
		}

		if err := s.dispatchCommand(ctx, cmd); err != nil {
			if ctx.Err() != nil {
				return
			}
			s.report(err)
			continue
		}
		delete(state.commands, id)
	}
}

func (s *Service) fireDue(ctx context.Context, state *effectState, now time.Time) {
	for ref, timeout := range state.timeouts {
		if timeout.data.At.After(now) || timeout.notBefore.After(now) {
			continue
		}

		if err := s.fireTimeout(ctx, timeout); err != nil {
			if ctx.Err() != nil {
				return
			}
			timeout.notBefore = now.Add(s.timerResolution)
			state.timeouts[ref] = timeout
			s.report(err)
			continue
		}

		delete(state.timeouts, ref)
	}
}

// dispatchCommand dispatches a pending command effect. The freshly fetched
// workflow decides whether the command is still pending: if another instance
// already dispatched it, the effect is dropped without a duplicate dispatch.
// The command goes over the bus at most once per call; recording the
// dispatch is retried against fresh state when a concurrent write to the
// workflow interferes. A crash between the bus dispatch and saving the
// dispatch record leaves the command pending, so command dispatch is
// at-least-once with a deterministic command id.
func (s *Service) dispatchCommand(ctx context.Context, cmd pendingCommand) error {
	def := s.defs[cmd.workflow]
	if def == nil {
		return fmt.Errorf("pending command for unknown workflow type %q", cmd.workflow)
	}

	defer s.locks.Lock(cmd.id)()

	var dispatched bool
	var err error
	for attempt := 0; attempt < processAttempts; attempt++ {
		var w Workflow
		var base *Base
		if w, base, err = s.fetchWorkflow(ctx, def, cmd.id); err != nil {
			err = fmt.Errorf("fetch %s workflow %s: %w", cmd.workflow, cmd.id, err)
			continue
		}

		data, ok := base.pendingCommand(cmd.effectID)
		if !ok {
			return nil
		}

		if !dispatched {
			if s.commandBus == nil {
				return errors.New("dispatch pending command: no command bus")
			}

			load, decodeErr := s.enc.Unmarshal(data.Payload, data.Name)
			if decodeErr != nil {
				return fmt.Errorf("decode %q command payload: %w", data.Name, decodeErr)
			}

			c := command.New(
				data.Name,
				load,
				command.ID(data.CommandID),
				command.Aggregate(data.AggregateName, data.AggregateID),
			)

			if dispatchErr := s.commandBus.Dispatch(ctx, c.Any()); dispatchErr != nil {
				return fmt.Errorf("dispatch %q command for %s workflow %s: %w", data.Name, cmd.workflow, cmd.id, dispatchErr)
			}
			dispatched = true
		}

		base.recordCommandDispatched(cmd.effectID)
		if err = s.save(ctx, def, w); err != nil {
			err = fmt.Errorf("mark %q command dispatched for %s workflow %s: %w", data.Name, cmd.workflow, cmd.id, err)
			continue
		}

		return nil
	}

	return err
}

// fireTimeout fires a due timeout by processing a TimeoutFired event through
// the trigger path. The trigger processing verifies against the fetched
// workflow that the timeout is still active before recording the fire and
// running handlers.
func (s *Service) fireTimeout(ctx context.Context, timeout activeTimeout) error {
	def := s.defs[timeout.workflow]
	if def == nil {
		return fmt.Errorf("active timeout for unknown workflow type %q", timeout.workflow)
	}

	evt := event.New(
		TimeoutFired,
		TimeoutFiredData{
			EffectID:     timeout.data.EffectID,
			WorkflowID:   timeout.id,
			Key:          timeout.data.Key,
			ScheduledFor: timeout.data.At,
		},
		event.ID(timeoutFireID(timeout.data.EffectID)),
	).Any()

	regs := def.registrations[TimeoutFired]
	indexes := make([]int, 0, len(regs))
	for i, reg := range regs {
		if id, ok := reg.correlate(evt); ok && id == timeout.id {
			indexes = append(indexes, i)
		}
	}

	return s.process(ctx, def, evt, timeout.id, indexes, false, true)
}
