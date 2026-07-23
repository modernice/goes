package workflow_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	aggrepo "github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/helper/streams"
	"github.com/modernice/goes/workflow"
)

const (
	orderPlacedEvent     = "test.order.placed"
	orderConfirmedEvent  = "test.order.confirmed"
	paymentDeclinedEvent = "test.order.payment_declined"
	stockReleasedEvent   = "test.order.stock_released"
	reserveStockCommand  = "test.reserve_stock"
	releaseStockCommand  = "test.release_stock"

	orderWorkflowName      = "test.order_workflow"
	orderPlacedRecorded    = "test.order_workflow.placed"
	orderConfirmedRecorded = "test.order_workflow.confirmed"
	orderTimedOutRecorded  = "test.order_workflow.timed_out"

	compWorkflowName               = "test.comp_workflow"
	compWorkflowPlacedRecorded     = "test.comp_workflow.placed"
	compWorkflowDeclinedRecorded   = "test.comp_workflow.declined"
	compWorkflowForwardRecorded    = "test.comp_workflow.stock_released.forward"
	compWorkflowCompensatingRecord = "test.comp_workflow.stock_released.compensating"
	compWorkflowTimeoutRecorded    = "test.comp_workflow.timeout"

	guardWorkflowName   = "test.guard_workflow"
	guardFiredRecorded  = "test.guard_workflow.fired"
	conflictWorkflowNme = "test.conflict_workflow"

	commandWaitTimeout      = 2 * time.Second
	stateWaitTimeout        = 2 * time.Second
	defaultTimerResolution  = 5 * time.Millisecond
	defaultDispatchInterval = 10 * time.Millisecond
)

type orderPlacedData struct {
	OrderID uuid.UUID
}

type orderConfirmedData struct {
	OrderID uuid.UUID
}

type paymentDeclinedData struct {
	OrderID uuid.UUID
}

type stockReleasedData struct {
	OrderID uuid.UUID
}

type reserveStockData struct {
	OrderID uuid.UUID
}

type releaseStockData struct {
	OrderID uuid.UUID
}

type placedRecordedData struct{}
type confirmedRecordedData struct{}
type timedOutRecordedData struct{}
type compPlacedRecordedData struct{}
type compDeclinedRecordedData struct{}
type compForwardRecordedData struct{}
type compCompensatingRecordedData struct{}
type compTimeoutRecordedData struct{}
type guardFiredRecordedData struct{}

type orderWorkflow struct {
	*workflow.Base

	paymentDelay time.Duration

	PlacedCount    int
	ConfirmedCount int
	TimedOutCount  int
	StartedApplied int
}

type compWorkflow struct {
	*workflow.Base

	releaseDelay time.Duration

	PlacedCount          int
	DeclinedCount        int
	ForwardReleasedCount int
	CompReleasedCount    int
	CompensationTimedOut int
}

type guardWorkflow struct {
	*workflow.Base

	timeoutDelay time.Duration

	FiredCount int
}

func newOrderWorkflowDefinition(paymentDelay time.Duration) (func(uuid.UUID) *orderWorkflow, workflow.Definition) {
	factory := func(id uuid.UUID) *orderWorkflow {
		w := &orderWorkflow{
			Base:         workflow.New(orderWorkflowName, id),
			paymentDelay: paymentDelay,
		}

		event.ApplyWith(w, w.applyStarted, workflow.Started)
		event.ApplyWith(w, w.applyPlaced, orderPlacedRecorded)
		event.ApplyWith(w, w.applyConfirmed, orderConfirmedRecorded)
		event.ApplyWith(w, w.applyTimedOut, orderTimedOutRecorded)

		return w
	}

	def := workflow.Define(
		factory,
		workflow.Starts(workflow.ByAggregateID, (*orderWorkflow).onPlaced, orderPlacedEvent),
		workflow.Reacts(workflow.ByAggregateID, (*orderWorkflow).onConfirmed, orderConfirmedEvent),
		workflow.OnTimeout("payment", (*orderWorkflow).onTimeout),
	)

	return factory, def
}

func newCompWorkflowDefinition(releaseDelay time.Duration) (func(uuid.UUID) *compWorkflow, workflow.Definition) {
	factory := func(id uuid.UUID) *compWorkflow {
		w := &compWorkflow{
			Base:         workflow.New(compWorkflowName, id),
			releaseDelay: releaseDelay,
		}

		event.ApplyWith(w, w.applyPlaced, compWorkflowPlacedRecorded)
		event.ApplyWith(w, w.applyDeclined, compWorkflowDeclinedRecorded)
		event.ApplyWith(w, w.applyForwardReleased, compWorkflowForwardRecorded)
		event.ApplyWith(w, w.applyCompReleased, compWorkflowCompensatingRecord)
		event.ApplyWith(w, w.applyCompTimeout, compWorkflowTimeoutRecorded)

		return w
	}

	def := workflow.Define(
		factory,
		workflow.Starts(workflow.ByAggregateID, (*compWorkflow).onPlaced, orderPlacedEvent),
		workflow.Reacts(workflow.ByAggregateID, (*compWorkflow).onPaymentDeclined, paymentDeclinedEvent),
		workflow.Reacts(workflow.ByAggregateID, (*compWorkflow).onForwardStockReleased, stockReleasedEvent),
		workflow.Compensates(workflow.ByAggregateID, (*compWorkflow).onCompensatingStockReleased, stockReleasedEvent),
		workflow.OnCompensationTimeout("release-stock", (*compWorkflow).onReleaseTimeout),
	)

	return factory, def
}

func newGuardWorkflowDefinition(timeoutDelay time.Duration) (func(uuid.UUID) *guardWorkflow, workflow.Definition) {
	factory := func(id uuid.UUID) *guardWorkflow {
		w := &guardWorkflow{
			Base:         workflow.New(guardWorkflowName, id),
			timeoutDelay: timeoutDelay,
		}

		event.ApplyWith(w, w.applyFired, guardFiredRecorded)

		return w
	}

	def := workflow.Define(
		factory,
		workflow.Starts(workflow.ByAggregateID, (*guardWorkflow).onPlaced, orderPlacedEvent),
		workflow.Reacts(workflow.ByAggregateID, (*guardWorkflow).onConfirmed, orderConfirmedEvent),
		workflow.OnTimeout("guard", (*guardWorkflow).onTimeout),
	)

	return factory, def
}

func (w *orderWorkflow) onPlaced(ctx workflow.Ctx[orderPlacedData]) error {
	aggregate.Next(w, orderPlacedRecorded, placedRecordedData{})

	if err := ctx.Dispatch("reserve", command.New(
		reserveStockCommand,
		reserveStockData{OrderID: ctx.Event().Data().OrderID},
		command.Aggregate("order", ctx.Event().Data().OrderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("payment", time.Now().Add(w.paymentDelay))
}

func (w *orderWorkflow) onConfirmed(ctx workflow.Ctx[orderConfirmedData]) error {
	aggregate.Next(w, orderConfirmedRecorded, confirmedRecordedData{})

	if err := ctx.Unschedule("payment"); err != nil {
		return err
	}

	return ctx.Complete()
}

func (w *orderWorkflow) onTimeout(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	aggregate.Next(w, orderTimedOutRecorded, timedOutRecordedData{})
	return ctx.Fail(errors.New("payment timeout"))
}

func (w *orderWorkflow) applyStarted(event.Of[workflow.StartedData]) { w.StartedApplied++ }
func (w *orderWorkflow) applyPlaced(event.Of[placedRecordedData])    { w.PlacedCount++ }
func (w *orderWorkflow) applyConfirmed(event.Of[confirmedRecordedData]) {
	w.ConfirmedCount++
}
func (w *orderWorkflow) applyTimedOut(event.Of[timedOutRecordedData]) { w.TimedOutCount++ }

func (w *compWorkflow) onPlaced(ctx workflow.Ctx[orderPlacedData]) error {
	aggregate.Next(w, compWorkflowPlacedRecorded, compPlacedRecordedData{})
	return ctx.Schedule("payment", time.Now().Add(time.Hour))
}

func (w *compWorkflow) onPaymentDeclined(ctx workflow.Ctx[paymentDeclinedData]) error {
	aggregate.Next(w, compWorkflowDeclinedRecorded, compDeclinedRecordedData{})

	if err := ctx.Compensate(errors.New("payment declined")); err != nil {
		return err
	}

	if err := ctx.Dispatch("release-stock", command.New(
		releaseStockCommand,
		releaseStockData{OrderID: ctx.Event().Data().OrderID},
		command.Aggregate("order", ctx.Event().Data().OrderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("release-stock", time.Now().Add(w.releaseDelay))
}

func (w *compWorkflow) onForwardStockReleased(ctx workflow.Ctx[stockReleasedData]) error {
	aggregate.Next(w, compWorkflowForwardRecorded, compForwardRecordedData{})
	return nil
}

func (w *compWorkflow) onCompensatingStockReleased(ctx workflow.Ctx[stockReleasedData]) error {
	aggregate.Next(w, compWorkflowCompensatingRecord, compCompensatingRecordedData{})
	return ctx.Compensated()
}

func (w *compWorkflow) onReleaseTimeout(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	aggregate.Next(w, compWorkflowTimeoutRecorded, compTimeoutRecordedData{})
	return ctx.CompensationFailed(errors.New("release-stock timeout"))
}

func (w *compWorkflow) applyPlaced(event.Of[compPlacedRecordedData])     { w.PlacedCount++ }
func (w *compWorkflow) applyDeclined(event.Of[compDeclinedRecordedData]) { w.DeclinedCount++ }
func (w *compWorkflow) applyForwardReleased(event.Of[compForwardRecordedData]) {
	w.ForwardReleasedCount++
}
func (w *compWorkflow) applyCompReleased(event.Of[compCompensatingRecordedData]) {
	w.CompReleasedCount++
}
func (w *compWorkflow) applyCompTimeout(event.Of[compTimeoutRecordedData]) {
	w.CompensationTimedOut++
}

func (w *guardWorkflow) onPlaced(ctx workflow.Ctx[orderPlacedData]) error {
	if err := ctx.Dispatch("marker", command.New(
		reserveStockCommand,
		reserveStockData{OrderID: ctx.Event().Data().OrderID},
		command.Aggregate("order", ctx.Event().Data().OrderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("guard", time.Now().Add(w.timeoutDelay))
}

func (w *guardWorkflow) onConfirmed(ctx workflow.Ctx[orderConfirmedData]) error {
	return ctx.Unschedule("guard")
}

func (w *guardWorkflow) onTimeout(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	aggregate.Next(w, guardFiredRecorded, guardFiredRecordedData{})
	return nil
}

func (w *guardWorkflow) applyFired(event.Of[guardFiredRecordedData]) { w.FiredCount++ }

type recordingCommandBus struct {
	*cmdbus.Bus

	mu         sync.Mutex
	dispatched []command.Command
	notify     chan command.Command
	cancel     context.CancelFunc
	done       chan struct{}
	errs       []error
}

func newCommandBus(t *testing.T, reg codec.Encoding) *recordingCommandBus {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	bus := cmdbus.New(reg, eventbus.New(), cmdbus.AssignTimeout(500*time.Millisecond))

	errs, err := bus.Run(ctx)
	if err != nil {
		cancel()
		t.Fatalf("run command bus: %v", err)
	}

	commands, subErrs, err := bus.Subscribe(ctx, reserveStockCommand, releaseStockCommand)
	if err != nil {
		cancel()
		t.Fatalf("subscribe command bus: %v", err)
	}

	rec := &recordingCommandBus{
		Bus:    bus,
		notify: make(chan command.Command, 32),
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		defer close(rec.done)
		for errs != nil || subErrs != nil || commands != nil {
			select {
			case err, ok := <-errs:
				if !ok {
					errs = nil
					continue
				}
				if err != nil && !ignorableCommandBusError(err) {
					rec.mu.Lock()
					rec.errs = append(rec.errs, err)
					rec.mu.Unlock()
				}
			case err, ok := <-subErrs:
				if !ok {
					subErrs = nil
					continue
				}
				if err != nil && !ignorableCommandBusError(err) {
					rec.mu.Lock()
					rec.errs = append(rec.errs, err)
					rec.mu.Unlock()
				}
			case cmdCtx, ok := <-commands:
				if !ok {
					commands = nil
					continue
				}
				cmd := command.Any(cmdCtx)
				rec.mu.Lock()
				rec.dispatched = append(rec.dispatched, cmd)
				rec.mu.Unlock()

				select {
				case rec.notify <- cmd:
				default:
				}
			}
		}
	}()

	return rec
}

func (b *recordingCommandBus) Close(t *testing.T) {
	t.Helper()
	b.cancel()
	<-b.done

	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.errs) > 0 {
		t.Fatalf("command bus returned errors: %v", b.errs)
	}
}

func (b *recordingCommandBus) Commands() []command.Command {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]command.Command, len(b.dispatched))
	copy(out, b.dispatched)
	return out
}

func ignorableCommandBusError(err error) bool {
	return errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled")
}

func countCommandsByName(commands []command.Command, name string) int {
	var count int
	for _, cmd := range commands {
		if cmd.Name() == name {
			count++
		}
	}
	return count
}

type failInsertStore struct {
	event.Store

	mu        sync.Mutex
	failName  string
	remaining int
}

func (s *failInsertStore) Insert(ctx context.Context, events ...event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.remaining > 0 {
		for _, evt := range events {
			if evt.Name() == s.failName {
				s.remaining--
				return errors.New("forced insert failure")
			}
		}
	}

	return s.Store.Insert(ctx, events...)
}

func TestService_StartReactAndIgnoreLateEvents(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	bus := eventbus.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderWorkflowDefinition(time.Second)

	svc := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         bus,
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	orderID := uuid.New()
	if err := bus.Publish(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("publish placed event: %v", err)
	}

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusRunning && w.PlacedCount == 1 && w.StartedApplied == 1
	})

	if err := bus.Publish(context.Background(), newOrderConfirmed(orderID, uuid.New())); err != nil {
		t.Fatalf("publish confirmed event: %v", err)
	}

	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusCompleted && w.ConfirmedCount == 1
	})

	before := countEvents(t, store, orderWorkflowName, orderID, workflow.TriggerRecorded)

	if err := bus.Publish(context.Background(), newOrderConfirmed(orderID, uuid.New())); err != nil {
		t.Fatalf("publish late confirmed event: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	after := countEvents(t, store, orderWorkflowName, orderID, workflow.TriggerRecorded)
	if after != before {
		t.Fatalf("late event should have been ignored; trigger records before=%d after=%d", before, after)
	}
}

func TestService_DuplicateTriggerIgnored(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	bus := eventbus.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderWorkflowDefinition(time.Second)

	svc := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         bus,
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	orderID := uuid.New()
	evt := newOrderPlaced(orderID, uuid.New())
	if err := bus.Publish(context.Background(), evt, evt); err != nil {
		t.Fatalf("publish duplicate placed event: %v", err)
	}

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		return countEvents(t, store, orderWorkflowName, orderID, workflow.CommandDispatched) == 1
	})

	if got := len(cmdBus.Commands()); got != 1 {
		t.Fatalf("expected 1 dispatched command; got %d", got)
	}

	if got := countEvents(t, store, orderWorkflowName, orderID, workflow.TriggerRecorded); got != 1 {
		t.Fatalf("expected 1 trigger record; got %d", got)
	}
}

func TestTrigger_ReplaysPendingCommandOnRestart(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderWorkflowDefinition(time.Second)
	orderID := uuid.New()

	svc := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
		CommandBus: cmdBus,
	}, def)
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("expected no command dispatch before runtime starts; got %d", got)
	}

	restarted := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		return countEvents(t, store, orderWorkflowName, orderID, workflow.CommandDispatched) == 1
	})
}

func TestRun_ReplaysTriggerHistoryOnStartup(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderWorkflowDefinition(time.Second)
	orderID := uuid.New()

	if err := store.Insert(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("insert placed event: %v", err)
	}

	svc := workflow.NewService(workflow.Config{
		Commands:            reg,
		EventStore:          store,
		EventBus:            eventbus.New(),
		CommandBus:          cmdBus,
		DispatchInterval:    defaultDispatchInterval,
		TimerResolution:     defaultTimerResolution,
		TriggerReplayWindow: time.Hour,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusRunning && w.PlacedCount == 1 && w.StartedApplied == 1
	})
}

func TestRun_DoesNotReplayTriggersWithoutWindow(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderWorkflowDefinition(time.Second)
	orderID := uuid.New()

	if err := store.Insert(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("insert placed event: %v", err)
	}

	svc := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	time.Sleep(100 * time.Millisecond)

	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("expected no commands without trigger replay; got %d", got)
	}

	w := loadWorkflow(t, store, factory, orderID)
	if version := w.AggregateVersion(); version != 0 {
		t.Fatalf("expected workflow to remain untouched; got version %d", version)
	}
}

func TestRun_DoesNotReplayTriggersOutsideWindow(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderWorkflowDefinition(time.Second)
	orderID := uuid.New()

	old := event.New(
		orderPlacedEvent,
		orderPlacedData{OrderID: orderID},
		event.ID(uuid.New()),
		event.Time(time.Now().Add(-48*time.Hour)),
		event.Aggregate(orderID, "order", 1),
	).Any()
	if err := store.Insert(context.Background(), old); err != nil {
		t.Fatalf("insert old placed event: %v", err)
	}

	svc := workflow.NewService(workflow.Config{
		Commands:            reg,
		EventStore:          store,
		EventBus:            eventbus.New(),
		CommandBus:          cmdBus,
		DispatchInterval:    defaultDispatchInterval,
		TimerResolution:     defaultTimerResolution,
		TriggerReplayWindow: time.Hour,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	time.Sleep(100 * time.Millisecond)

	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("expected no commands from old trigger replay; got %d", got)
	}

	w := loadWorkflow(t, store, factory, orderID)
	if version := w.AggregateVersion(); version != 0 {
		t.Fatalf("expected workflow to remain untouched; got version %d", version)
	}
}

func TestCommandRecovery_RedispatchesSameCommandIDAfterSaveFailure(t *testing.T) {
	reg := newRegistry()
	underlying := eventstore.New()
	// The runtime retries recording a dispatch against fresh state, so the
	// insert must fail once per attempt to surface the error.
	store := &failInsertStore{
		Store:     underlying,
		failName:  workflow.CommandDispatched,
		remaining: 3,
	}
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderWorkflowDefinition(time.Second)
	orderID := uuid.New()

	svc := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: time.Second,
		TimerResolution:  defaultTimerResolution,
	}, def)
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errs, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("run service: %v", err)
	}

	awaitCommand(t, cmdBus, 1)

	var gotErr error
	select {
	case gotErr = <-errs:
	case <-time.After(commandWaitTimeout):
		t.Fatal("timed out waiting for dispatch persistence failure")
	}
	cancel()
	for range errs {
	}

	if gotErr == nil {
		t.Fatal("expected command dispatch persistence failure")
	}

	restarted := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       underlying,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 2)

	commands := cmdBus.Commands()
	if len(commands) != 2 {
		t.Fatalf("expected 2 dispatched commands; got %d", len(commands))
	}

	if commands[0].ID() != commands[1].ID() {
		t.Fatalf("expected redispatched command to reuse command id; got %s and %s", commands[0].ID(), commands[1].ID())
	}
}

func TestCommandRecovery_DoesNotRedispatchDispatchedCommand(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderWorkflowDefinition(time.Second)
	orderID := uuid.New()

	svc := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	cancel, wait := runService(t, svc)
	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		return countEvents(t, store, orderWorkflowName, orderID, workflow.CommandDispatched) == 1
	})
	cancel()
	wait()

	restarted := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait = runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	time.Sleep(100 * time.Millisecond)

	if got := len(cmdBus.Commands()); got != 1 {
		t.Fatalf("expected no redispatch of an already dispatched command; got %d dispatches", got)
	}
}

func TestTimeoutRecovery_FiresAfterRestart(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderWorkflowDefinition(20 * time.Millisecond)
	orderID := uuid.New()

	svc := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
		CommandBus: cmdBus,
	}, def)
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	restarted := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusFailed && w.TimedOutCount == 1 && w.Reason() == "payment timeout"
	})

	if got := countEvents(t, store, orderWorkflowName, orderID, workflow.TimeoutFired); got != 1 {
		t.Fatalf("expected 1 timeout fired event; got %d", got)
	}
}

func TestTimeout_CanceledByOtherInstanceDoesNotFire(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newGuardWorkflowDefinition(1500 * time.Millisecond)
	orderID := uuid.New()

	// The writer plays the role of another service instance that shares the
	// event store but not the event bus.
	writer := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
	}, def)

	if err := writer.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	runner := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
		ResyncInterval:   -1,
	}, def)

	cancel, wait := runService(t, runner)
	defer func() {
		cancel()
		wait()
	}()

	// Once the marker command was dispatched, the runner has recovered the
	// timeout into its in-memory state. The cancel below may race the
	// runner's dispatch record; the store rejects one of the writes and the
	// runtime retries it against fresh state.
	awaitCommand(t, cmdBus, 1)

	// Cancel the timeout through the writer; the runner never learns about
	// the cancellation because periodic resyncs are disabled.
	if err := writer.Trigger(context.Background(), newOrderConfirmed(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger confirmed event: %v", err)
	}

	time.Sleep(1700 * time.Millisecond)

	if got := countEvents(t, store, guardWorkflowName, orderID, workflow.TimeoutFired); got != 0 {
		t.Fatalf("canceled timeout should not have fired; got %d timeout fired events", got)
	}

	w := loadWorkflow(t, store, factory, orderID)
	if w.FiredCount != 0 {
		t.Fatalf("timeout handler should not have run; got %d runs", w.FiredCount)
	}
	if w.Status() != workflow.StatusRunning {
		t.Fatalf("expected workflow to still be running; got %q", w.Status())
	}
}

func TestResync_PicksUpEffectsFromOtherInstances(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderWorkflowDefinition(time.Hour)
	orderID := uuid.New()

	runner := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
		ResyncInterval:   25 * time.Millisecond,
	}, def)

	cancel, wait := runService(t, runner)
	defer func() {
		cancel()
		wait()
	}()

	// The writer records a command effect that never reaches the runner via
	// notification or trigger; only the periodic resync can discover it.
	writer := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
	}, def)
	if err := writer.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	// The takeover grace holds the freshly discovered effect back at first:
	// the instance that recorded it keeps a head start.
	time.Sleep(250 * time.Millisecond)
	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("expected the takeover grace to delay the dispatch; got %d commands", got)
	}

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		return countEvents(t, store, orderWorkflowName, orderID, workflow.CommandDispatched) == 1
	})
}

func TestRecoveryWindow(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderWorkflowDefinition(time.Hour)
	orderID := uuid.New()

	writer := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
	}, def)
	if err := writer.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	// Let the recorded command effect age beyond the recovery window.
	time.Sleep(250 * time.Millisecond)

	bounded := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
		ResyncInterval:   -1,
		RecoveryWindow:   100 * time.Millisecond,
	}, def)

	cancel, wait := runService(t, bounded)

	// Wait well past the takeover grace: the effect must not be recovered,
	// because it was recorded before the recovery window.
	time.Sleep(600 * time.Millisecond)
	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("expected effect outside the recovery window to be ignored; got %d commands", got)
	}
	cancel()
	wait()

	// A negative window scans the full history and recovers the effect.
	unbounded := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
		ResyncInterval:   -1,
		RecoveryWindow:   -1,
	}, def)

	cancel, wait = runService(t, unbounded)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
}

func TestService_NoDefinitions(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderWorkflowDefinition(time.Second)

	// Populate the store with workflow effect events of another service.
	writer := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
	}, def)
	if err := writer.Trigger(context.Background(), newOrderPlaced(uuid.New(), uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	empty := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	})

	cancel, wait := runService(t, empty)

	time.Sleep(100 * time.Millisecond)

	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("service without definitions should not dispatch commands; got %d", got)
	}

	cancel()
	wait()
}

func TestDispatch_EffectConflict(t *testing.T) {
	t.Run("same command is idempotent", func(t *testing.T) {
		reg := newRegistry()
		store := eventstore.New()

		type conflictWorkflow struct{ *workflow.Base }
		factory := func(id uuid.UUID) *conflictWorkflow {
			return &conflictWorkflow{Base: workflow.New(conflictWorkflowNme, id)}
		}

		def := workflow.Define(
			factory,
			workflow.Starts(workflow.ByAggregateID, func(w *conflictWorkflow, ctx workflow.Ctx[orderPlacedData]) error {
				cmd := func() command.Command {
					return command.New(
						reserveStockCommand,
						reserveStockData{OrderID: ctx.Event().Data().OrderID},
						command.Aggregate("order", ctx.Event().Data().OrderID),
					).Any()
				}

				if err := ctx.Dispatch("reserve", cmd()); err != nil {
					return err
				}
				return ctx.Dispatch("reserve", cmd())
			}, orderPlacedEvent),
		)

		svc := workflow.NewService(workflow.Config{Commands: reg, EventStore: store}, def)
		orderID := uuid.New()
		if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
			t.Fatalf("trigger placed event: %v", err)
		}

		if got := countEvents(t, store, conflictWorkflowNme, orderID, workflow.CommandRequested); got != 1 {
			t.Fatalf("expected 1 command requested event; got %d", got)
		}
	})

	t.Run("different command conflicts", func(t *testing.T) {
		reg := newRegistry()
		store := eventstore.New()

		type conflictWorkflow struct{ *workflow.Base }
		factory := func(id uuid.UUID) *conflictWorkflow {
			return &conflictWorkflow{Base: workflow.New(conflictWorkflowNme, id)}
		}

		def := workflow.Define(
			factory,
			workflow.Starts(workflow.ByAggregateID, func(w *conflictWorkflow, ctx workflow.Ctx[orderPlacedData]) error {
				if err := ctx.Dispatch("reserve", command.New(
					reserveStockCommand,
					reserveStockData{OrderID: ctx.Event().Data().OrderID},
					command.Aggregate("order", ctx.Event().Data().OrderID),
				).Any()); err != nil {
					return err
				}

				return ctx.Dispatch("reserve", command.New(
					reserveStockCommand,
					reserveStockData{OrderID: uuid.New()},
					command.Aggregate("order", uuid.New()),
				).Any())
			}, orderPlacedEvent),
		)

		svc := workflow.NewService(workflow.Config{Commands: reg, EventStore: store}, def)
		err := svc.Trigger(context.Background(), newOrderPlaced(uuid.New(), uuid.New()))
		if !errors.Is(err, workflow.ErrEffectConflict) {
			t.Fatalf("expected ErrEffectConflict; got %v", err)
		}
	})
}

func TestCompensationLifecycleAndPhaseRouting(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	bus := eventbus.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newCompWorkflowDefinition(time.Second)

	svc := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         bus,
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	orderID := uuid.New()
	if err := bus.Publish(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("publish placed event: %v", err)
	}

	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusRunning && w.PlacedCount == 1
	})

	if err := bus.Publish(context.Background(), newStockReleased(orderID, uuid.New())); err != nil {
		t.Fatalf("publish forward stock released event: %v", err)
	}

	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusRunning && w.ForwardReleasedCount == 1 && w.CompReleasedCount == 0
	})

	if err := bus.Publish(context.Background(), newPaymentDeclined(orderID, uuid.New())); err != nil {
		t.Fatalf("publish payment declined event: %v", err)
	}

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusCompensating &&
			w.DeclinedCount == 1 &&
			w.Reason() == "payment declined"
	})

	if got := countEvents(t, store, compWorkflowName, orderID, workflow.CompensationStarted); got != 1 {
		t.Fatalf("expected 1 compensation started event; got %d", got)
	}

	if got := countEvents(t, store, compWorkflowName, orderID, workflow.TimeoutCanceled); got != 1 {
		t.Fatalf("expected 1 timeout canceled event after entering compensation; got %d", got)
	}

	if err := bus.Publish(context.Background(), newStockReleased(orderID, uuid.New())); err != nil {
		t.Fatalf("publish compensating stock released event: %v", err)
	}

	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusCompensated &&
			w.ForwardReleasedCount == 1 &&
			w.CompReleasedCount == 1 &&
			w.Reason() == "payment declined"
	})

	if got := countEvents(t, store, compWorkflowName, orderID, workflow.CompensationCompleted); got != 1 {
		t.Fatalf("expected 1 compensation completed event; got %d", got)
	}

	if got := countEvents(t, store, compWorkflowName, orderID, workflow.TimeoutCanceled); got != 2 {
		t.Fatalf("expected 2 timeout canceled events after compensation completed; got %d", got)
	}

	if got := countCommandsByName(cmdBus.Commands(), releaseStockCommand); got != 1 {
		t.Fatalf("expected 1 release-stock command; got %d", got)
	}
}

func TestCompensation_DuplicateTriggerIgnored(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	bus := eventbus.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newCompWorkflowDefinition(time.Second)

	svc := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         bus,
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	orderID := uuid.New()
	if err := bus.Publish(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("publish placed event: %v", err)
	}

	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusRunning
	})

	evt := newPaymentDeclined(orderID, uuid.New())
	if err := bus.Publish(context.Background(), evt, evt); err != nil {
		t.Fatalf("publish duplicate declined event: %v", err)
	}

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusCompensating && w.DeclinedCount == 1
	})

	if got := countEvents(t, store, compWorkflowName, orderID, workflow.CompensationStarted); got != 1 {
		t.Fatalf("expected 1 compensation started event; got %d", got)
	}

	if got := countCommandsByName(cmdBus.Commands(), releaseStockCommand); got != 1 {
		t.Fatalf("expected 1 release-stock command after duplicate trigger; got %d", got)
	}
}

func TestCompensationRecovery_ReplaysPendingCommandOnRestart(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newCompWorkflowDefinition(time.Second)
	orderID := uuid.New()

	svc := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
		CommandBus: cmdBus,
	}, def)
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}
	if err := svc.Trigger(context.Background(), newPaymentDeclined(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger declined event: %v", err)
	}

	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("expected no commands before runtime starts; got %d", got)
	}

	restarted := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusCompensating && w.Reason() == "payment declined"
	})

	if got := countCommandsByName(cmdBus.Commands(), releaseStockCommand); got != 1 {
		t.Fatalf("expected 1 release-stock command after restart; got %d", got)
	}
}

func TestCompensationTimeoutRecovery_FailsAfterRestart(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newCompWorkflowDefinition(20 * time.Millisecond)
	orderID := uuid.New()

	svc := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
		CommandBus: cmdBus,
	}, def)
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}
	if err := svc.Trigger(context.Background(), newPaymentDeclined(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger declined event: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	restarted := workflow.NewService(workflow.Config{
		Commands:         reg,
		EventStore:       store,
		EventBus:         eventbus.New(),
		CommandBus:       cmdBus,
		DispatchInterval: defaultDispatchInterval,
		TimerResolution:  defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		w := loadWorkflow(t, store, factory, orderID)
		return w.Status() == workflow.StatusFailed &&
			w.CompensationTimedOut == 1 &&
			w.Reason() == "release-stock timeout"
	})

	if got := countEvents(t, store, compWorkflowName, orderID, workflow.CompensationFailed); got != 1 {
		t.Fatalf("expected 1 compensation failed event; got %d", got)
	}

	if got := countEvents(t, store, compWorkflowName, orderID, workflow.TimeoutFired); got != 1 {
		t.Fatalf("expected 1 compensation timeout fired event; got %d", got)
	}
}

func TestCompensationTransitionValidation(t *testing.T) {
	t.Run("compensated outside compensation", func(t *testing.T) {
		reg := newRegistry()
		store := eventstore.New()

		type invalidWorkflow struct{ *workflow.Base }
		factory := func(id uuid.UUID) *invalidWorkflow {
			return &invalidWorkflow{Base: workflow.New("test.invalid_compensated", id)}
		}

		def := workflow.Define(
			factory,
			workflow.Starts(workflow.ByAggregateID, func(w *invalidWorkflow, ctx workflow.Ctx[orderPlacedData]) error {
				return ctx.Compensated()
			}, orderPlacedEvent),
		)

		svc := workflow.NewService(workflow.Config{Commands: reg, EventStore: store}, def)
		err := svc.Trigger(context.Background(), newOrderPlaced(uuid.New(), uuid.New()))
		if !errors.Is(err, workflow.ErrNotCompensating) {
			t.Fatalf("expected ErrNotCompensating; got %v", err)
		}
	})

	t.Run("fail while compensating", func(t *testing.T) {
		reg := newRegistry()
		store := eventstore.New()

		type invalidWorkflow struct{ *workflow.Base }
		factory := func(id uuid.UUID) *invalidWorkflow {
			return &invalidWorkflow{Base: workflow.New("test.invalid_transition", id)}
		}

		def := workflow.Define(
			factory,
			workflow.Starts(workflow.ByAggregateID, func(w *invalidWorkflow, ctx workflow.Ctx[orderPlacedData]) error {
				return nil
			}, orderPlacedEvent),
			workflow.Reacts(workflow.ByAggregateID, func(w *invalidWorkflow, ctx workflow.Ctx[paymentDeclinedData]) error {
				if err := ctx.Compensate(errors.New("boom")); err != nil {
					return err
				}
				return ctx.Fail(errors.New("still boom"))
			}, paymentDeclinedEvent),
		)

		svc := workflow.NewService(workflow.Config{Commands: reg, EventStore: store}, def)
		orderID := uuid.New()
		if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
			t.Fatalf("trigger placed event: %v", err)
		}

		err := svc.Trigger(context.Background(), newPaymentDeclined(orderID, uuid.New()))
		if !errors.Is(err, workflow.ErrInvalidTransition) {
			t.Fatalf("expected ErrInvalidTransition; got %v", err)
		}
	})
}

func TestRegisterEvents(t *testing.T) {
	reg := newRegistry()
	want := workflow.TimeoutRequestedData{
		EffectID:  uuid.New(),
		Key:       "payment",
		TriggerID: uuid.New(),
		At:        time.Now().UTC().Truncate(0),
	}

	payload, err := reg.Marshal(want)
	if err != nil {
		t.Fatalf("marshal timeout request: %v", err)
	}

	gotAny, err := reg.Unmarshal(payload, workflow.TimeoutRequested)
	if err != nil {
		t.Fatalf("unmarshal timeout request: %v", err)
	}

	got, ok := gotAny.(workflow.TimeoutRequestedData)
	if !ok {
		t.Fatalf("expected workflow.TimeoutRequestedData; got %T", gotAny)
	}

	if got != want {
		t.Fatalf("decoded payload mismatch: want=%#v got=%#v", want, got)
	}

	compWant := workflow.CompensationStartedData{Reason: "boom"}
	compPayload, err := reg.Marshal(compWant)
	if err != nil {
		t.Fatalf("marshal compensation started: %v", err)
	}

	compAny, err := reg.Unmarshal(compPayload, workflow.CompensationStarted)
	if err != nil {
		t.Fatalf("unmarshal compensation started: %v", err)
	}

	compGot, ok := compAny.(workflow.CompensationStartedData)
	if !ok {
		t.Fatalf("expected workflow.CompensationStartedData; got %T", compAny)
	}

	if compGot != compWant {
		t.Fatalf("decoded compensation payload mismatch: want=%#v got=%#v", compWant, compGot)
	}
}

func newRegistry() *codec.Registry {
	reg := codec.New()
	workflow.RegisterEvents(reg)
	codec.Register[orderPlacedData](reg, orderPlacedEvent)
	codec.Register[orderConfirmedData](reg, orderConfirmedEvent)
	codec.Register[paymentDeclinedData](reg, paymentDeclinedEvent)
	codec.Register[stockReleasedData](reg, stockReleasedEvent)
	codec.Register[reserveStockData](reg, reserveStockCommand)
	codec.Register[releaseStockData](reg, releaseStockCommand)
	codec.Register[placedRecordedData](reg, orderPlacedRecorded)
	codec.Register[confirmedRecordedData](reg, orderConfirmedRecorded)
	codec.Register[timedOutRecordedData](reg, orderTimedOutRecorded)
	codec.Register[compPlacedRecordedData](reg, compWorkflowPlacedRecorded)
	codec.Register[compDeclinedRecordedData](reg, compWorkflowDeclinedRecorded)
	codec.Register[compForwardRecordedData](reg, compWorkflowForwardRecorded)
	codec.Register[compCompensatingRecordedData](reg, compWorkflowCompensatingRecord)
	codec.Register[compTimeoutRecordedData](reg, compWorkflowTimeoutRecorded)
	codec.Register[guardFiredRecordedData](reg, guardFiredRecorded)
	return reg
}

func newOrderPlaced(orderID, triggerID uuid.UUID) event.Event {
	return event.New(
		orderPlacedEvent,
		orderPlacedData{OrderID: orderID},
		event.ID(triggerID),
		event.Aggregate(orderID, "order", 1),
	).Any()
}

func newOrderConfirmed(orderID, triggerID uuid.UUID) event.Event {
	return event.New(
		orderConfirmedEvent,
		orderConfirmedData{OrderID: orderID},
		event.ID(triggerID),
		event.Aggregate(orderID, "order", 2),
	).Any()
}

func newPaymentDeclined(orderID, triggerID uuid.UUID) event.Event {
	return event.New(
		paymentDeclinedEvent,
		paymentDeclinedData{OrderID: orderID},
		event.ID(triggerID),
		event.Aggregate(orderID, "order", 3),
	).Any()
}

func newStockReleased(orderID, triggerID uuid.UUID) event.Event {
	return event.New(
		stockReleasedEvent,
		stockReleasedData{OrderID: orderID},
		event.ID(triggerID),
		event.Aggregate(orderID, "order", 4),
	).Any()
}

func runService(t *testing.T, svc *workflow.Service) (context.CancelFunc, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	errs, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("run workflow service: %v", err)
	}

	var (
		mu   sync.Mutex
		list []error
		done = make(chan struct{})
	)

	go func() {
		defer close(done)
		for err := range errs {
			if err == nil {
				continue
			}
			mu.Lock()
			list = append(list, err)
			mu.Unlock()
		}
	}()

	wait := func() {
		t.Helper()
		<-done
		mu.Lock()
		defer mu.Unlock()
		if len(list) > 0 {
			t.Fatalf("service returned errors: %v", list)
		}
	}

	return cancel, wait
}

func awaitCommand(t *testing.T, bus *recordingCommandBus, want int) {
	t.Helper()

	deadline := time.After(commandWaitTimeout)
	for {
		if got := len(bus.Commands()); got >= want {
			return
		}

		select {
		case <-bus.notify:
		case <-deadline:
			t.Fatalf("timed out waiting for %d dispatched commands; got %d", want, len(bus.Commands()))
		}
	}
}

func awaitState(t *testing.T, ok func() bool) {
	t.Helper()

	deadline := time.Now().Add(stateWaitTimeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for workflow state")
}

func loadWorkflow[W workflow.Workflow](t *testing.T, store event.Store, factory func(uuid.UUID) W, id uuid.UUID) W {
	t.Helper()

	repo := aggrepo.New(store)
	w := factory(id)
	if err := repo.Fetch(context.Background(), w); err != nil {
		t.Fatalf("fetch workflow %s: %v", id, err)
	}
	return w
}

func countEvents(t *testing.T, store event.Store, aggregateName string, workflowID uuid.UUID, names ...string) int {
	t.Helper()

	events, errs, err := store.Query(context.Background(), query.New(
		query.Name(names...),
		query.AggregateName(aggregateName),
		query.AggregateID(workflowID),
		query.SortByTime(),
	))
	if err != nil {
		t.Fatalf("query workflow events %v: %v", names, err)
	}

	all, err := streams.Drain(context.Background(), events, errs)
	if err != nil {
		t.Fatalf("drain workflow events %v: %v", names, err)
	}

	return len(all)
}
