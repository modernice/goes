package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/handler"
	"github.com/modernice/goes/event/query"
	qtime "github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/concurrent"
)

const (
	defaultDispatchInterval = 100 * time.Millisecond
	defaultTimerResolution  = 25 * time.Millisecond
	defaultResyncInterval   = time.Minute
	defaultRecoveryWindow   = 30 * 24 * time.Hour

	// processAttempts bounds how often a trigger is re-processed when saving
	// the workflow fails, typically because another instance concurrently
	// modified it. Each attempt re-fetches the workflow, so retried triggers
	// are deduplicated against the newly persisted state.
	processAttempts = 3

	updateBufferSize = 1024
)

// namespace is the UUID namespace for deterministic effect, command, and
// timeout-fire ids.
var namespace = uuid.MustParse("3f1d97cf-9a4b-4d55-8ec7-62c1b2d0f5aa")

// Config configures a Service.
type Config struct {
	// EventStore is the event store that persists workflow instances.
	EventStore event.Store

	// EventBus is the event bus that delivers trigger events. Required if
	// any definition registers trigger events.
	EventBus event.Bus

	// CommandBus is the command bus that dispatches recorded command
	// effects.
	CommandBus command.Bus

	// Commands encodes and decodes command payloads. If it implements
	// codec.Registerer, the built-in workflow events are registered into it
	// as well — pass the codec registry of the application here.
	Commands codec.Encoding

	// NewRepository constructs the aggregate repository that fetches and
	// saves workflow instances (default: repository.New(EventStore)).
	// Definitions that bring their own repository (see WithRepository) are
	// unaffected. The constructed repository must persist to EventStore.
	NewRepository func(event.Store) aggregate.Repository

	// Strict makes the Service report an error when a trigger event
	// correlates to an unknown workflow, instead of silently ignoring it.
	Strict bool

	// Workers is the number of concurrent trigger workers (default 1).
	// Trigger processing is synchronized per workflow, so multiple workers
	// are safe.
	Workers int

	// DispatchInterval is the interval at which pending commands are
	// dispatched and retried (default 100ms).
	DispatchInterval time.Duration

	// TimerResolution is the resolution of the timeout timer (default 25ms).
	TimerResolution time.Duration

	// TriggerReplayWindow enables the replay of trigger events from the
	// event store when the service starts: trigger events no older than the
	// window are re-processed, which recovers workflows that missed events
	// while no service was running — useful with non-durable event buses
	// such as NATS Core. Zero (the default) disables the replay. Already
	// handled triggers are deduplicated, so generous windows are safe.
	TriggerReplayWindow time.Duration

	// ResyncInterval is the interval at which the effect runtime re-reads
	// effect events from the event store (default 1m; a negative duration
	// disables periodic resyncs). Resyncs are the liveness backstop of the
	// effect runtime: they discover effects that were recorded by other
	// service instances — letting any instance take over the pending
	// commands and timeouts of a crashed one — and recover local effect
	// notifications that were dropped under load. Without periodic resyncs,
	// effects recorded by other instances are only picked up at startup.
	//
	// Effects discovered through resyncs are attempted only once they are
	// older than a short takeover grace, giving the instance that recorded
	// them a head start and avoiding duplicate dispatches between healthy
	// instances.
	ResyncInterval time.Duration

	// RecoveryWindow bounds how far the initial effect recovery reaches back
	// into the event store when the service starts (default 30 days; a
	// negative duration scans the full history). It keeps the startup scan
	// proportional to recent activity instead of the total history of the
	// store. Effects recorded before the window are not recovered after a
	// restart, so the window must exceed the longest timeout horizon of the
	// workflows plus the maximum expected downtime.
	RecoveryWindow time.Duration
}

// Service runs distributed workflow instances on top of an event bus and
// event store. Trigger events from the bus start and advance workflows;
// recorded effects (commands and timeouts) are executed by a background
// runtime that survives restarts by recovering from the event store.
//
// The error channel returned by Run must be drained; an undrained channel
// eventually pauses the runtime.
type Service struct {
	handler *handler.Handler

	enc        codec.Encoding
	store      event.Store
	commandBus command.Bus
	repo       aggregate.Repository
	strict     bool

	dispatchInterval time.Duration
	timerResolution  time.Duration
	replayWindow     time.Duration
	resyncInterval   time.Duration
	recoveryWindow   time.Duration

	defs     map[string]*definition
	triggers map[string][]*definition
	repos    map[string]*workflowRepository
	names    []string

	updates chan event.Event
	locks   keyedMutex

	errs chan error
	fail func(error)
}

// NewService returns a new workflow service that runs the given workflow
// definitions.
func NewService(cfg Config, defs ...Definition) *Service {
	enc := cfg.Commands
	if enc == nil {
		enc = command.NewRegistry()
	}
	if r, ok := enc.(codec.Registerer); ok {
		RegisterEvents(r)
	}

	repo := aggregate.Repository(nil)
	if cfg.NewRepository != nil {
		repo = cfg.NewRepository(cfg.EventStore)
	} else {
		repo = repository.New(cfg.EventStore)
	}

	s := &Service{
		enc:              enc,
		store:            cfg.EventStore,
		commandBus:       cfg.CommandBus,
		repo:             repo,
		strict:           cfg.Strict,
		dispatchInterval: defaultDispatchInterval,
		timerResolution:  defaultTimerResolution,
		replayWindow:     cfg.TriggerReplayWindow,
		resyncInterval:   defaultResyncInterval,
		recoveryWindow:   defaultRecoveryWindow,
		defs:             make(map[string]*definition),
		triggers:         make(map[string][]*definition),
		repos:            make(map[string]*workflowRepository),
		updates:          make(chan event.Event, updateBufferSize),
	}

	if cfg.DispatchInterval > 0 {
		s.dispatchInterval = cfg.DispatchInterval
	}
	if cfg.TimerResolution > 0 {
		s.timerResolution = cfg.TimerResolution
	}
	if cfg.ResyncInterval != 0 {
		s.resyncInterval = cfg.ResyncInterval
	}
	if cfg.RecoveryWindow != 0 {
		s.recoveryWindow = cfg.RecoveryWindow
	}

	for _, def := range defs {
		s.addDefinition(def)
	}

	if len(s.triggers) > 0 && cfg.EventBus != nil {
		opts := []handler.Option{}
		if cfg.Workers > 0 {
			opts = append(opts, handler.Workers(cfg.Workers))
		}
		if cfg.EventStore != nil && s.replayWindow > 0 {
			opts = append(opts,
				handler.Startup(cfg.EventStore),
				handler.StartupQuery(func(q event.Query) event.Query {
					return query.Merge(q, query.New(
						query.Time(qtime.After(time.Now().Add(-s.replayWindow))),
					))
				}),
			)
		}

		s.handler = handler.New(cfg.EventBus, opts...)
		for name := range s.triggers {
			s.handler.RegisterEventHandler(name, s.handleTrigger)
		}
	}

	return s
}

// Run starts the workflow runtime. It subscribes to the registered trigger
// events, optionally replays recent triggers from the event store
// (Config.TriggerReplayWindow), and starts the effect runtime that
// dispatches commands and fires timeouts.
//
// Callers must drain the returned error channel.
func (s *Service) Run(ctx context.Context) (<-chan error, error) {
	if len(s.triggers) > 0 && s.handler == nil {
		return nil, errors.New("workflow service: no event bus")
	}

	s.errs, s.fail = concurrent.Errors(ctx)

	out := []<-chan error{s.errs}
	if s.handler != nil {
		errs, err := s.handler.Run(ctx)
		if err != nil {
			return nil, fmt.Errorf("run workflow trigger handler: %w", err)
		}
		out = append(out, errs)
	}

	if len(s.defs) > 0 {
		go s.runEffects(ctx)
	}

	return streams.FanInAll(out...), nil
}

// Trigger processes an event through the same trigger path that is used for
// events from the event bus. It is mainly useful in tests and for manually
// feeding events into the workflow runtime.
func (s *Service) Trigger(ctx context.Context, evt event.Event) error {
	defs := s.triggers[evt.Name()]
	if len(defs) == 0 {
		return nil
	}

	var errs []error
	for _, def := range defs {
		if err := s.triggerDefinition(ctx, def, evt); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (s *Service) triggerDefinition(ctx context.Context, def *definition, evt event.Event) error {
	type group struct {
		indexes []int
		start   bool
	}

	regs := def.registrations[evt.Name()]
	groups := make(map[uuid.UUID]group)
	for i, reg := range regs {
		id, ok := reg.correlate(evt)
		if !ok {
			continue
		}

		if id == uuid.Nil {
			return fmt.Errorf("%s workflow correlated %q event to nil workflow id", def.name, evt.Name())
		}

		g := groups[id]
		g.indexes = append(g.indexes, i)
		g.start = g.start || reg.start
		groups[id] = g
	}

	var errs []error
	for id, g := range groups {
		if err := s.process(ctx, def, evt, id, g.indexes, g.start, false); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (s *Service) handleTrigger(evt event.Event) {
	ctx := context.Background()
	if s.handler != nil {
		if runCtx := s.handler.Context(); runCtx != nil {
			ctx = runCtx
		}
	}

	if err := s.Trigger(ctx, evt); err != nil {
		s.report(fmt.Errorf("handle workflow triggers: %w", err))
	}
}

// process handles a trigger event for a single workflow instance. When
// saving the workflow fails — typically because another instance
// concurrently modified it — the trigger is re-processed against the fresh
// state, up to processAttempts times.
func (s *Service) process(
	ctx context.Context,
	def *definition,
	evt event.Event,
	id uuid.UUID,
	indexes []int,
	allowCreate bool,
	fire bool,
) error {
	defer s.locks.Lock(id)()

	var err error
	for attempt := 0; attempt < processAttempts; attempt++ {
		var retry bool
		if retry, err = s.processOnce(ctx, def, evt, id, indexes, allowCreate, fire); err == nil || !retry || ctx.Err() != nil {
			return err
		}
	}
	return err
}

func (s *Service) processOnce(
	ctx context.Context,
	def *definition,
	evt event.Event,
	id uuid.UUID,
	indexes []int,
	allowCreate bool,
	fire bool,
) (retry bool, _ error) {
	w, base, err := s.fetchWorkflow(ctx, def, id)
	if err != nil {
		return true, fmt.Errorf("fetch %s workflow %s: %w", def.name, id, err)
	}

	_, _, version := w.Aggregate()
	exists := version > 0

	if !exists && !allowCreate {
		if s.strict || fire {
			return false, fmt.Errorf("unknown %s workflow %s for %q event", def.name, id, evt.Name())
		}
		return false, nil
	}

	if base.handledTrigger(evt.ID()) || base.Done() {
		return false, nil
	}

	// TimeoutFired events act exclusively through the timeout runtime, which
	// verifies against the fetched workflow that the timeout is still
	// active. Copies that arrive over the event bus or through startup
	// replay were already processed and are dropped here.
	if !fire && evt.Name() == TimeoutFired {
		return false, nil
	}

	if !exists {
		base.recordStarted()
	}

	if fire {
		data, err := decodeTimeoutFired(evt)
		if err != nil {
			return false, err
		}

		// The fetched workflow is the authority on whether the timeout is
		// still active: it may have been canceled or rescheduled — possibly
		// by another service instance — since the timeout runtime last
		// synced with the store.
		if _, active := base.activeTimeoutByID(data.EffectID); !active {
			return false, nil
		}

		base.recordTimeoutFired(data, evt.ID())
	}

	regs := def.registrations[evt.Name()]
	matching := matchPhase(regs, base.compensating(), indexes)
	if len(matching) == 0 {
		if len(w.AggregateChanges()) == 0 {
			return false, nil
		}
		if err := s.save(ctx, def, w); err != nil {
			return true, err
		}
		return false, nil
	}

	base.recordTrigger(evt)

	for _, idx := range matching {
		rt := handlerRuntime{
			Context:  ctx,
			workflow: w,
			base:     base,
			trigger:  evt,
			enc:      s.enc,
		}

		if err := regs[idx].handle(rt); err != nil {
			return false, fmt.Errorf("handle %q event in %s workflow %s: %w", evt.Name(), def.name, id, err)
		}
	}

	if err := s.save(ctx, def, w); err != nil {
		return true, err
	}
	return false, nil
}

// matchPhase filters the correlated trigger registrations down to those that
// match the current phase of the workflow: Compensates registrations run
// while compensating, all others while running.
func matchPhase(regs []triggerRegistration, compensating bool, indexes []int) []int {
	matching := make([]int, 0, len(indexes))
	for _, idx := range indexes {
		if idx < 0 || idx >= len(regs) {
			continue
		}
		if (regs[idx].phase == phaseCompensating) == compensating {
			matching = append(matching, idx)
		}
	}
	return matching
}

func (s *Service) save(ctx context.Context, def *definition, w Workflow) error {
	changes := append([]event.Event(nil), w.AggregateChanges()...)
	if len(changes) == 0 {
		return nil
	}

	if err := s.repos[def.name].save(ctx, w); err != nil {
		return err
	}

	for _, evt := range changes {
		switch evt.Name() {
		case CommandRequested, CommandDispatched, TimeoutRequested, TimeoutCanceled, TimeoutFired:
			s.notify(evt)
		}
	}

	return nil
}

// notify forwards a persisted effect event to the effect runtime. It never
// blocks: if the buffer is full (or the runtime is not running), the event
// is dropped and picked up by the next resync from the event store.
func (s *Service) notify(evt event.Event) {
	select {
	case s.updates <- evt:
	default:
	}
}

func (s *Service) report(err error) {
	if err == nil || errors.Is(err, context.Canceled) || s.fail == nil {
		return
	}
	s.fail(err)
}

func (s *Service) addDefinition(def Definition) {
	if def.def == nil {
		panic("workflow.NewService: nil definition")
	}

	typed := def.def
	if _, ok := s.defs[typed.name]; ok {
		panic(fmt.Sprintf("workflow.NewService: duplicate workflow aggregate name %q", typed.name))
	}

	s.defs[typed.name] = typed
	s.names = append(s.names, typed.name)
	s.repos[typed.name] = s.resolveRepository(typed)

	for eventName := range typed.registrations {
		s.triggers[eventName] = append(s.triggers[eventName], typed)
	}
}

// resolveRepository returns the repository of a definition, falling back to
// the default repository of the Service. The resolved repository is stored
// on the Service, never on the definition, because definitions are shared
// between services.
func (s *Service) resolveRepository(def *definition) *workflowRepository {
	if def.repo != nil {
		return def.repo
	}

	return &workflowRepository{
		fetch: func(ctx context.Context, id uuid.UUID) (Workflow, error) {
			w := s.newWorkflow(def, id)
			if err := s.repo.Fetch(ctx, w); err != nil {
				return nil, err
			}
			return w, nil
		},
		save: func(ctx context.Context, w Workflow) error {
			return s.repo.Save(ctx, w)
		},
	}
}

// fetchWorkflow fetches the current state of a workflow through the
// repository of its definition.
func (s *Service) fetchWorkflow(ctx context.Context, def *definition, id uuid.UUID) (Workflow, *Base, error) {
	w, err := s.repos[def.name].fetch(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	base := w.workflowBase()
	if base == nil {
		panic(fmt.Sprintf("workflow service: %q repository returned workflow with nil base", def.name))
	}

	return w, base, nil
}

func (s *Service) newWorkflow(def *definition, id uuid.UUID) Workflow {
	if def == nil || def.create == nil {
		panic("workflow service: nil definition")
	}

	w := def.create(id)
	if w == nil {
		panic(fmt.Sprintf("workflow service: %q constructor returned nil", def.name))
	}

	if w.workflowBase() == nil {
		panic(fmt.Sprintf("workflow service: %q instance has nil workflow base", def.name))
	}
	return w
}

// effectID derives the deterministic id of an effect from the workflow, the
// trigger event, and the effect key, making effect recording idempotent
// across retries of the same trigger.
func effectID(kind string, workflowID, triggerID uuid.UUID, key string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte(kind+":"+workflowID.String()+":"+triggerID.String()+":"+key))
}

func commandID(effectID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte("command:"+effectID.String()))
}

func timeoutFireID(effectID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte("timeout-fired:"+effectID.String()))
}
