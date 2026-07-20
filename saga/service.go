package saga

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	aggrepo "github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/event"
	evhandler "github.com/modernice/goes/event/handler"
	"github.com/modernice/goes/event/query"
	qtime "github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/internal/concurrent"
)

const (
	defaultRetryInterval   = 100 * time.Millisecond
	defaultTimerResolution = 25 * time.Millisecond
	defaultTriggerReplay   = 24 * time.Hour
	runtimeBufferSize      = 1024
)

var sagaNamespace = uuid.MustParse("a83004d8-68c4-4f45-8683-f6ee7c0f08d2")

// Instance is a concrete saga instance.
type Instance interface {
	aggregate.Aggregate
	SagaBase() *Base
}

// Config configures a Service.
type Config struct {
	Encoding        codec.Encoding
	Store           event.Store
	Bus             event.Bus
	Commands        command.Bus
	Strict          bool
	RetryInterval   time.Duration
	TimerResolution time.Duration

	// TriggerReplayWindow bounds startup replay of trigger events from the event store.
	// Zero uses the default window of 24h. A negative duration disables trigger replay.
	TriggerReplayWindow time.Duration
}

// Service runs distributed saga instances on top of an event bus and event store.
type Service struct {
	*evhandler.Handler

	enc                 codec.Encoding
	store               event.Store
	cmdBus              command.Bus
	repo                *aggrepo.Repository
	strict              bool
	retryInterval       time.Duration
	timerResolution     time.Duration
	triggerReplayWindow time.Duration

	byName    map[string]*definition
	byTrigger map[string][]*definition
	names     []string

	commandUpdates chan event.Event
	timeoutUpdates chan event.Event

	lockMux  sync.Mutex
	sagaLock map[uuid.UUID]*sync.Mutex

	errs chan error
	fail func(error)
}

type pendingCommand struct {
	sagaName string
	sagaID   uuid.UUID
	data     CommandRequestedData
}

type activeTimeout struct {
	sagaName    string
	sagaID      uuid.UUID
	data        TimeoutRequestedData
	nextAttempt time.Time
}

type handlerGroup struct {
	indexes []int
	start   bool
}

// NewService returns a new saga service.
func NewService(cfg Config, defs ...Definition) *Service {
	enc := cfg.Encoding
	if enc == nil {
		enc = command.NewRegistry()
	}
	if r, ok := enc.(codec.Registerer); ok {
		RegisterEvents(r)
	}

	s := &Service{
		enc:                 enc,
		store:               cfg.Store,
		cmdBus:              cfg.Commands,
		repo:                aggrepo.New(cfg.Store),
		strict:              cfg.Strict,
		retryInterval:       defaultRetryInterval,
		timerResolution:     defaultTimerResolution,
		triggerReplayWindow: defaultTriggerReplay,
		byName:              make(map[string]*definition),
		byTrigger:           make(map[string][]*definition),
		commandUpdates:      make(chan event.Event, runtimeBufferSize),
		timeoutUpdates:      make(chan event.Event, runtimeBufferSize),
		sagaLock:            make(map[uuid.UUID]*sync.Mutex),
	}

	if cfg.RetryInterval > 0 {
		s.retryInterval = cfg.RetryInterval
	}
	if cfg.TimerResolution > 0 {
		s.timerResolution = cfg.TimerResolution
	}
	if cfg.TriggerReplayWindow != 0 {
		s.triggerReplayWindow = cfg.TriggerReplayWindow
	}

	for _, def := range defs {
		s.addDefinition(def)
	}

	if len(s.byTrigger) > 0 && cfg.Bus != nil {
		opts := []evhandler.Option{}
		if cfg.Store != nil && s.triggerReplayWindow >= 0 {
			opts = append(opts, evhandler.Startup(cfg.Store))
			if s.triggerReplayWindow > 0 {
				opts = append(opts, evhandler.StartupQuery(func(q event.Query) event.Query {
					return query.Merge(q, query.New(
						query.Time(qtime.After(time.Now().Add(-s.triggerReplayWindow))),
					))
				}))
			}
		}

		s.Handler = evhandler.New(cfg.Bus, opts...)
		for name := range s.byTrigger {
			s.Handler.RegisterEventHandler(name, s.handleTrigger)
		}
	}

	return s
}

// Run starts the saga runtime.
func (s *Service) Run(ctx context.Context) (<-chan error, error) {
	if len(s.byTrigger) > 0 && s.Handler == nil {
		return nil, errors.New("saga service: missing event bus")
	}

	s.errs, s.fail = concurrent.Errors(ctx)

	errChans := []<-chan error{s.errs}
	if s.Handler != nil {
		errs, err := s.Handler.Run(ctx)
		if err != nil {
			return nil, fmt.Errorf("run saga trigger handler: %w", err)
		}
		errChans = append(errChans, errs)
	}

	go s.runCommandWorker(ctx, s.fail)
	go s.runTimeoutWorker(ctx, s.fail)

	return streams.FanInAll(errChans...), nil
}

// Inject processes an event through the same trigger path used by the bus runtime.
func (s *Service) Inject(ctx context.Context, evt event.Event) error {
	defs := s.byTrigger[evt.Name()]
	if len(defs) == 0 {
		return nil
	}

	var errs []error
	for _, def := range defs {
		if err := s.injectIntoDefinition(ctx, def, evt); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (s *Service) injectIntoDefinition(ctx context.Context, def *definition, evt event.Event) error {
	regs := def.registrations[evt.Name()]
	groups := make(map[uuid.UUID]handlerGroup)
	for i, reg := range regs {
		id, ok := reg.correlate(evt)
		if !ok {
			continue
		}

		if id == uuid.Nil {
			return fmt.Errorf("%s saga correlated %q event to nil saga id", def.name, evt.Name())
		}

		group := groups[id]
		group.indexes = append(group.indexes, i)
		group.start = group.start || reg.start
		groups[id] = group
	}

	var errs []error
	for id, group := range groups {
		if err := s.processDefinitionGroup(ctx, def, evt, id, group.indexes, group.start, false); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (s *Service) handleTrigger(evt event.Event) {
	ctx := context.Background()
	if s.Handler != nil {
		if runCtx := s.Handler.Context(); runCtx != nil {
			ctx = runCtx
		}
	}

	if err := s.Inject(ctx, evt); err != nil {
		s.pushError(fmt.Errorf("handle saga triggers: %w", err))
	}
}

func (s *Service) pushError(err error) {
	if err == nil || errors.Is(err, context.Canceled) || s.fail == nil {
		return
	}

	s.fail(err)
}

func (s *Service) processDefinitionGroup(
	ctx context.Context,
	def *definition,
	evt event.Event,
	sagaID uuid.UUID,
	indexes []int,
	allowCreate bool,
	persistInput bool,
) error {
	return s.withSagaLock(sagaID, func() error {
		inst := s.newInstance(def, sagaID)
		base := inst.SagaBase()

		if err := s.repo.Fetch(ctx, inst); err != nil {
			return fmt.Errorf("fetch %s saga %s: %w", def.name, sagaID, err)
		}

		_, _, version := inst.Aggregate()
		exists := version > 0
		if !exists && !allowCreate {
			if s.strict || persistInput {
				return fmt.Errorf("unknown %s saga %s for %q event", def.name, sagaID, evt.Name())
			}
			return nil
		}

		if base.handledTrigger(evt.ID()) {
			return nil
		}

		if base.terminal() {
			return nil
		}

		if !exists {
			base.recordStarted()
		}

		if persistInput {
			data, err := decodeTimeoutFired(evt)
			if err != nil {
				return err
			}
			base.recordTimeoutFired(data, evt.ID())
		}

		matching := s.matchPhase(def, evt.Name(), base, indexes)
		if len(matching) == 0 {
			if len(inst.AggregateChanges()) == 0 {
				return nil
			}
			return s.save(ctx, inst)
		}

		base.recordTrigger(evt)

		regs := def.registrations[evt.Name()]
		for _, idx := range matching {
			if idx < 0 || idx >= len(regs) {
				return fmt.Errorf("invalid trigger index %d for %s/%q", idx, def.name, evt.Name())
			}

			rt := handlerRuntime{
				Context: ctx,
				saga:    inst,
				base:    base,
				trigger: evt,
				enc:     s.enc,
			}

			if err := regs[idx].handle(rt); err != nil {
				return fmt.Errorf("handle %s saga %s with %q event: %w", def.name, sagaID, evt.Name(), err)
			}
		}

		return s.save(ctx, inst)
	})
}

func (s *Service) matchPhase(def *definition, eventName string, base *Base, indexes []int) []int {
	regs := def.registrations[eventName]
	matching := make([]int, 0, len(indexes))
	for _, idx := range indexes {
		if idx < 0 || idx >= len(regs) {
			continue
		}

		reg := regs[idx]
		switch reg.phase {
		case phaseCompensating:
			if base.compensating() {
				matching = append(matching, idx)
			}
		default:
			if !base.compensating() {
				matching = append(matching, idx)
			}
		}
	}

	return matching
}

func (s *Service) save(ctx context.Context, inst Instance) error {
	changes := append([]event.Event(nil), inst.AggregateChanges()...)
	if len(changes) == 0 {
		return nil
	}

	if err := s.repo.Save(ctx, inst); err != nil {
		return err
	}

	for _, evt := range changes {
		switch evt.Name() {
		case CommandRequested, CommandDispatched:
			s.commandUpdates <- evt
		case TimeoutRequested, TimeoutCanceled, TimeoutFired:
			s.timeoutUpdates <- evt
		}
	}

	return nil
}

func (s *Service) runCommandWorker(ctx context.Context, fail func(error)) {
	pending := make(map[uuid.UUID]pendingCommand)
	if err := s.recoverCommands(ctx, pending); err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		fail(err)
	}

	ticker := time.NewTicker(s.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-s.commandUpdates:
			s.applyCommandUpdate(evt, pending)
		case <-ticker.C:
			for id, req := range pending {
				if err := s.dispatchPendingCommand(ctx, req); err != nil {
					if ctx.Err() != nil || errors.Is(err, context.Canceled) {
						return
					}
					fail(err)
					continue
				}
				delete(pending, id)
			}
		}
	}
}

func (s *Service) runTimeoutWorker(ctx context.Context, fail func(error)) {
	active := make(map[string]activeTimeout)
	if err := s.recoverTimeouts(ctx, active); err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		fail(err)
	}

	ticker := time.NewTicker(s.timerResolution)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-s.timeoutUpdates:
			s.applyTimeoutUpdate(evt, active)
		case now := <-ticker.C:
			for key, timeout := range active {
				if timeout.data.At.After(now) || timeout.nextAttempt.After(now) {
					continue
				}

				if err := s.fireTimeout(ctx, timeout); err != nil {
					if ctx.Err() != nil || errors.Is(err, context.Canceled) {
						return
					}
					timeout.nextAttempt = now.Add(s.timerResolution)
					active[key] = timeout
					fail(err)
					continue
				}

				delete(active, key)
			}
		}
	}
}

func (s *Service) recoverCommands(ctx context.Context, pending map[uuid.UUID]pendingCommand) error {
	events, errs, err := s.store.Query(ctx, query.New(
		query.Name(CommandRequested, CommandDispatched),
		query.AggregateName(s.names...),
		query.SortBy(event.SortTime, event.SortAsc),
	))
	if err != nil {
		return fmt.Errorf("query pending commands: %w", err)
	}

	return streams.Walk(ctx, func(evt event.Event) error {
		switch evt.Name() {
		case CommandRequested:
			data, ok := event.TryCast[CommandRequestedData](evt)
			if !ok {
				return fmt.Errorf("cast %q event to saga.CommandRequestedData", evt.Name())
			}
			id, name, _ := evt.Aggregate()
			pending[data.Data().EffectID] = pendingCommand{
				sagaName: name,
				sagaID:   id,
				data:     data.Data(),
			}
		case CommandDispatched:
			data, ok := event.TryCast[CommandDispatchedData](evt)
			if !ok {
				return fmt.Errorf("cast %q event to saga.CommandDispatchedData", evt.Name())
			}
			delete(pending, data.Data().EffectID)
		}
		return nil
	}, events, errs)
}

func (s *Service) recoverTimeouts(ctx context.Context, active map[string]activeTimeout) error {
	events, errs, err := s.store.Query(ctx, query.New(
		query.Name(TimeoutRequested, TimeoutCanceled, TimeoutFired),
		query.AggregateName(s.names...),
		query.SortBy(event.SortTime, event.SortAsc),
	))
	if err != nil {
		return fmt.Errorf("query active timeouts: %w", err)
	}

	return streams.Walk(ctx, func(evt event.Event) error {
		id, name, _ := evt.Aggregate()
		switch evt.Name() {
		case TimeoutRequested:
			data, ok := event.TryCast[TimeoutRequestedData](evt)
			if !ok {
				return fmt.Errorf("cast %q event to saga.TimeoutRequestedData", evt.Name())
			}
			timeout := activeTimeout{
				sagaName: name,
				sagaID:   id,
				data:     data.Data(),
			}
			active[timeoutMapKey(id, data.Data().Key)] = timeout
		case TimeoutCanceled:
			data, ok := event.TryCast[TimeoutCanceledData](evt)
			if !ok {
				return fmt.Errorf("cast %q event to saga.TimeoutCanceledData", evt.Name())
			}
			key := timeoutMapKey(id, data.Data().Key)
			if current, ok := active[key]; ok && current.data.EffectID == data.Data().EffectID {
				delete(active, key)
			}
		case TimeoutFired:
			data, ok := event.TryCast[TimeoutFiredData](evt)
			if !ok {
				return fmt.Errorf("cast %q event to saga.TimeoutFiredData", evt.Name())
			}
			delete(active, timeoutMapKey(id, data.Data().Key))
		}
		return nil
	}, events, errs)
}

func (s *Service) dispatchPendingCommand(ctx context.Context, req pendingCommand) error {
	if s.cmdBus == nil {
		return errors.New("dispatch pending command: missing command bus")
	}

	return s.withSagaLock(req.sagaID, func() error {
		load, err := s.enc.Unmarshal(req.data.Payload, req.data.Name)
		if err != nil {
			return fmt.Errorf("decode %q command payload: %w", req.data.Name, err)
		}

		cmd := command.New(
			req.data.Name,
			load,
			command.ID(req.data.CommandID),
			command.Aggregate(req.data.AggregateName, req.data.AggregateID),
		)

		if err := s.cmdBus.Dispatch(ctx, cmd.Any()); err != nil {
			return fmt.Errorf("dispatch %q command for %s saga %s: %w", req.data.Name, req.sagaName, req.sagaID, err)
		}

		def := s.byName[req.sagaName]
		inst := s.newInstance(def, req.sagaID)
		if err := s.repo.Fetch(ctx, inst); err != nil {
			return fmt.Errorf("fetch %s saga %s after command dispatch: %w", req.sagaName, req.sagaID, err)
		}
		if !inst.SagaBase().hasPendingCommand(req.data.EffectID) {
			return nil
		}

		inst.SagaBase().recordCommandDispatched(req.data.EffectID)
		if err := s.save(ctx, inst); err != nil {
			return fmt.Errorf("mark %q command dispatched for %s saga %s: %w", req.data.Name, req.sagaName, req.sagaID, err)
		}

		return nil
	})
}

func (s *Service) fireTimeout(ctx context.Context, timeout activeTimeout) error {
	def := s.byName[timeout.sagaName]
	if def == nil {
		return fmt.Errorf("fire timeout for unknown saga type %q", timeout.sagaName)
	}

	evt := event.New(
		TimeoutFired,
		TimeoutFiredData{
			EffectID:     timeout.data.EffectID,
			SagaID:       timeout.sagaID,
			Key:          timeout.data.Key,
			ScheduledFor: timeout.data.At,
		},
		event.ID(timeoutFireID(timeout.data.EffectID)),
	)

	fired := evt.Any()
	regs := def.registrations[TimeoutFired]
	indexes := make([]int, 0, len(regs))
	for i, reg := range regs {
		id, ok := reg.correlate(fired)
		if ok && id == timeout.sagaID {
			indexes = append(indexes, i)
		}
	}

	return s.processDefinitionGroup(ctx, def, fired, timeout.sagaID, indexes, false, true)
}

func (s *Service) applyCommandUpdate(evt event.Event, pending map[uuid.UUID]pendingCommand) {
	switch evt.Name() {
	case CommandRequested:
		data, ok := event.TryCast[CommandRequestedData](evt)
		if !ok {
			return
		}
		id, name, _ := evt.Aggregate()
		pending[data.Data().EffectID] = pendingCommand{
			sagaName: name,
			sagaID:   id,
			data:     data.Data(),
		}
	case CommandDispatched:
		data, ok := event.TryCast[CommandDispatchedData](evt)
		if !ok {
			return
		}
		delete(pending, data.Data().EffectID)
	}
}

func (s *Service) applyTimeoutUpdate(evt event.Event, active map[string]activeTimeout) {
	id, name, _ := evt.Aggregate()
	switch evt.Name() {
	case TimeoutRequested:
		data, ok := event.TryCast[TimeoutRequestedData](evt)
		if !ok {
			return
		}
		active[timeoutMapKey(id, data.Data().Key)] = activeTimeout{
			sagaName: name,
			sagaID:   id,
			data:     data.Data(),
		}
	case TimeoutCanceled:
		data, ok := event.TryCast[TimeoutCanceledData](evt)
		if !ok {
			return
		}
		key := timeoutMapKey(id, data.Data().Key)
		if current, ok := active[key]; ok && current.data.EffectID == data.Data().EffectID {
			delete(active, key)
		}
	case TimeoutFired:
		data, ok := event.TryCast[TimeoutFiredData](evt)
		if !ok {
			return
		}
		delete(active, timeoutMapKey(id, data.Data().Key))
	}
}

func (s *Service) addDefinition(def Definition) *definition {
	if def.def == nil {
		panic("saga.NewService: nil definition")
	}

	typed := def.def

	if typed.name == "" {
		panic("saga.NewService: empty saga aggregate name")
	}
	if _, ok := s.byName[typed.name]; ok {
		panic(fmt.Sprintf("saga.NewService: duplicate saga aggregate name %q", typed.name))
	}

	s.byName[typed.name] = typed
	s.names = append(s.names, typed.name)

	for eventName := range typed.registrations {
		s.byTrigger[eventName] = append(s.byTrigger[eventName], typed)
	}

	return typed
}

func (s *Service) newInstance(def *definition, id uuid.UUID) Instance {
	if def == nil || def.new == nil {
		panic("saga service: nil definition")
	}

	inst := def.new(id)
	if inst == nil {
		panic(fmt.Sprintf("saga service: %q constructor returned nil", def.name))
	}

	base := inst.SagaBase()
	if base == nil {
		panic(fmt.Sprintf("saga service: %q instance has nil saga base", def.name))
	}
	return inst
}

func effectID(kind string, sagaID, triggerID uuid.UUID, key string) uuid.UUID {
	return uuid.NewSHA1(sagaNamespace, []byte(kind+":"+sagaID.String()+":"+triggerID.String()+":"+key))
}

func commandID(effectID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(sagaNamespace, []byte("command:"+effectID.String()))
}

func timeoutFireID(effectID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(sagaNamespace, []byte("timeout-fired:"+effectID.String()))
}

func timeoutMapKey(sagaID uuid.UUID, key string) string {
	return sagaID.String() + ":" + key
}

func (s *Service) withSagaLock(id uuid.UUID, fn func() error) error {
	mu := s.sagaMutex(id)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

func (s *Service) sagaMutex(id uuid.UUID) *sync.Mutex {
	s.lockMux.Lock()
	defer s.lockMux.Unlock()

	if mu, ok := s.sagaLock[id]; ok {
		return mu
	}

	mu := &sync.Mutex{}
	s.sagaLock[id] = mu
	return mu
}
