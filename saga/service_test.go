package saga_test

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/modernice/goes/saga"
)

const (
	orderPlacedEvent               = "test.order.placed"
	orderConfirmedEvent            = "test.order.confirmed"
	paymentDeclinedEvent           = "test.order.payment_declined"
	stockReleasedEvent             = "test.order.stock_released"
	reserveStockCommand            = "test.reserve_stock"
	releaseStockCommand            = "test.release_stock"
	orderPlacedRecorded            = "test.order_saga.placed"
	orderConfirmedRecorded         = "test.order_saga.confirmed"
	orderTimedOutRecorded          = "test.order_saga.timed_out"
	orderSagaAggregateName         = "test.order_saga"
	compSagaPlacedRecorded         = "test.comp_saga.placed"
	compSagaDeclinedRecorded       = "test.comp_saga.declined"
	compSagaForwardReleaseRecorded = "test.comp_saga.stock_released.forward"
	compSagaCompReleaseRecorded    = "test.comp_saga.stock_released.compensating"
	compSagaTimeoutRecorded        = "test.comp_saga.timeout"
	compSagaAggregateName          = "test.comp_saga"
	commandWaitTimeout             = 2 * time.Second
	stateWaitTimeout               = 2 * time.Second
	defaultTimerResolution         = 5 * time.Millisecond
	defaultCommandRetry            = 10 * time.Millisecond
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
type compensationPlacedRecordedData struct{}
type compensationDeclinedRecordedData struct{}
type compensationForwardReleaseRecordedData struct{}
type compensationCompensatedReleaseRecordedData struct{}
type compensationTimeoutRecordedData struct{}

type orderSaga struct {
	*saga.Base

	paymentDelay time.Duration

	PlacedCount    int
	ConfirmedCount int
	TimedOutCount  int
	StartedApplied int
}

type compensationSaga struct {
	*saga.Base

	releaseDelay time.Duration

	PlacedCount          int
	DeclinedCount        int
	ForwardReleasedCount int
	CompReleasedCount    int
	CompensationTimedOut int
}

func newOrderSagaDefinition(paymentDelay time.Duration) (func(uuid.UUID) *orderSaga, saga.Definition) {
	factory := func(id uuid.UUID) *orderSaga {
		s := &orderSaga{
			Base:         saga.New(orderSagaAggregateName, id),
			paymentDelay: paymentDelay,
		}

		event.ApplyWith(s, s.applyStarted, saga.Started)
		event.ApplyWith(s, s.applyPlaced, orderPlacedRecorded)
		event.ApplyWith(s, s.applyConfirmed, orderConfirmedRecorded)
		event.ApplyWith(s, s.applyTimedOut, orderTimedOutRecorded)

		return s
	}

	def := saga.Define(
		factory,
		saga.Starts(saga.AggregateID[orderPlacedData](), (*orderSaga).onPlaced, orderPlacedEvent),
		saga.Reacts(saga.AggregateID[orderConfirmedData](), (*orderSaga).onConfirmed, orderConfirmedEvent),
		saga.OnTimeout("payment", (*orderSaga).onTimeout),
	)

	return factory, def
}

func newCompensationSagaDefinition(releaseDelay time.Duration) (func(uuid.UUID) *compensationSaga, saga.Definition) {
	factory := func(id uuid.UUID) *compensationSaga {
		s := &compensationSaga{
			Base:         saga.New(compSagaAggregateName, id),
			releaseDelay: releaseDelay,
		}

		event.ApplyWith(s, s.applyPlaced, compSagaPlacedRecorded)
		event.ApplyWith(s, s.applyDeclined, compSagaDeclinedRecorded)
		event.ApplyWith(s, s.applyForwardReleased, compSagaForwardReleaseRecorded)
		event.ApplyWith(s, s.applyCompReleased, compSagaCompReleaseRecorded)
		event.ApplyWith(s, s.applyCompTimeout, compSagaTimeoutRecorded)

		return s
	}

	def := saga.Define(
		factory,
		saga.Starts(saga.AggregateID[orderPlacedData](), (*compensationSaga).onPlaced, orderPlacedEvent),
		saga.Reacts(saga.AggregateID[paymentDeclinedData](), (*compensationSaga).onPaymentDeclined, paymentDeclinedEvent),
		saga.Reacts(saga.AggregateID[stockReleasedData](), (*compensationSaga).onForwardStockReleased, stockReleasedEvent),
		saga.Compensates(saga.AggregateID[stockReleasedData](), (*compensationSaga).onCompensatingStockReleased, stockReleasedEvent),
		saga.OnCompensationTimeout("release-stock", (*compensationSaga).onReleaseTimeout),
	)

	return factory, def
}

func (s *orderSaga) onPlaced(ctx saga.Ctx[orderPlacedData]) error {
	aggregate.Next(s, orderPlacedRecorded, placedRecordedData{})

	if err := ctx.Dispatch("reserve", command.New(
		reserveStockCommand,
		reserveStockData{OrderID: ctx.Event().Data().OrderID},
		command.Aggregate("order", ctx.Event().Data().OrderID),
	).Any()); err != nil {
		return err
	}

	return ctx.Schedule("payment", time.Now().Add(s.paymentDelay))
}

func (s *orderSaga) onConfirmed(ctx saga.Ctx[orderConfirmedData]) error {
	aggregate.Next(s, orderConfirmedRecorded, confirmedRecordedData{})

	if err := ctx.CancelTimeout("payment"); err != nil {
		return err
	}

	return ctx.Complete()
}

func (s *orderSaga) onTimeout(ctx saga.Ctx[saga.TimeoutFiredData]) error {
	aggregate.Next(s, orderTimedOutRecorded, timedOutRecordedData{})
	return ctx.Fail(errors.New("payment timeout"))
}

func (s *orderSaga) applyStarted(event.Of[saga.StartedData]) {
	s.StartedApplied++
}

func (s *orderSaga) applyPlaced(event.Of[placedRecordedData]) {
	s.PlacedCount++
}

func (s *orderSaga) applyConfirmed(event.Of[confirmedRecordedData]) {
	s.ConfirmedCount++
}

func (s *orderSaga) applyTimedOut(event.Of[timedOutRecordedData]) {
	s.TimedOutCount++
}

func (s *compensationSaga) onPlaced(ctx saga.Ctx[orderPlacedData]) error {
	aggregate.Next(s, compSagaPlacedRecorded, compensationPlacedRecordedData{})
	return ctx.Schedule("payment", time.Now().Add(time.Hour))
}

func (s *compensationSaga) onPaymentDeclined(ctx saga.Ctx[paymentDeclinedData]) error {
	aggregate.Next(s, compSagaDeclinedRecorded, compensationDeclinedRecordedData{})

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

	return ctx.Schedule("release-stock", time.Now().Add(s.releaseDelay))
}

func (s *compensationSaga) onForwardStockReleased(ctx saga.Ctx[stockReleasedData]) error {
	aggregate.Next(s, compSagaForwardReleaseRecorded, compensationForwardReleaseRecordedData{})
	return nil
}

func (s *compensationSaga) onCompensatingStockReleased(ctx saga.Ctx[stockReleasedData]) error {
	aggregate.Next(s, compSagaCompReleaseRecorded, compensationCompensatedReleaseRecordedData{})
	return ctx.Compensated()
}

func (s *compensationSaga) onReleaseTimeout(ctx saga.Ctx[saga.TimeoutFiredData]) error {
	aggregate.Next(s, compSagaTimeoutRecorded, compensationTimeoutRecordedData{})
	return ctx.CompensationFailed(errors.New("release-stock timeout"))
}

func (s *compensationSaga) applyPlaced(event.Of[compensationPlacedRecordedData]) {
	s.PlacedCount++
}

func (s *compensationSaga) applyDeclined(event.Of[compensationDeclinedRecordedData]) {
	s.DeclinedCount++
}

func (s *compensationSaga) applyForwardReleased(event.Of[compensationForwardReleaseRecordedData]) {
	s.ForwardReleasedCount++
}

func (s *compensationSaga) applyCompReleased(event.Of[compensationCompensatedReleaseRecordedData]) {
	s.CompReleasedCount++
}

func (s *compensationSaga) applyCompTimeout(event.Of[compensationTimeoutRecordedData]) {
	s.CompensationTimedOut++
}

type recordingCommandBus struct {
	*cmdbus.Bus[int]

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
	bus := cmdbus.New[int](reg, eventbus.New(), cmdbus.AssignTimeout(500*time.Millisecond))

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
	factory, def := newOrderSagaDefinition(time.Second)

	svc := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             bus,
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
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
		s := loadSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusRunning && s.PlacedCount == 1 && s.StartedApplied == 1
	})

	if err := bus.Publish(context.Background(), newOrderConfirmed(orderID, uuid.New())); err != nil {
		t.Fatalf("publish confirmed event: %v", err)
	}

	awaitState(t, func() bool {
		s := loadSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusCompleted && s.ConfirmedCount == 1
	})

	before := countSagaEvents(t, store, orderID, saga.TriggerRecorded)

	if err := bus.Publish(context.Background(), newOrderConfirmed(orderID, uuid.New())); err != nil {
		t.Fatalf("publish late confirmed event: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	after := countSagaEvents(t, store, orderID, saga.TriggerRecorded)
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
	_, def := newOrderSagaDefinition(time.Second)

	svc := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             bus,
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	orderID := uuid.New()
	triggerID := uuid.New()
	evt := newOrderPlaced(orderID, triggerID)
	if err := bus.Publish(context.Background(), evt, evt); err != nil {
		t.Fatalf("publish duplicate placed event: %v", err)
	}

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		return countSagaEvents(t, store, orderID, saga.CommandDispatched) == 1
	})

	if got := len(cmdBus.Commands()); got != 1 {
		t.Fatalf("expected 1 dispatched command; got %d", got)
	}

	if got := countSagaEvents(t, store, orderID, saga.TriggerRecorded); got != 1 {
		t.Fatalf("expected 1 trigger record; got %d", got)
	}
}

func TestInject_ReplaysPendingCommandOnRestart(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderSagaDefinition(time.Second)
	orderID := uuid.New()

	svc := saga.NewService(saga.Config{
		Encoding: reg,
		Store:    store,
		Commands: cmdBus,
	}, def)
	if err := svc.Inject(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("inject placed event: %v", err)
	}

	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("expected no command dispatch before runtime starts; got %d", got)
	}

	restarted := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             eventbus.New(),
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		return countSagaEvents(t, store, orderID, saga.CommandDispatched) == 1
	})
}

func TestRun_ReplaysTriggerHistoryOnStartup(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderSagaDefinition(time.Second)
	orderID := uuid.New()

	if err := store.Insert(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("insert placed event: %v", err)
	}

	svc := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             eventbus.New(),
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, svc)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		s := loadSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusRunning && s.PlacedCount == 1 && s.StartedApplied == 1
	})
}

func TestRun_DoesNotReplayTriggersOutsideWindow(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderSagaDefinition(time.Second)
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

	svc := saga.NewService(saga.Config{
		Encoding:            reg,
		Store:               store,
		Bus:                 eventbus.New(),
		Commands:            cmdBus,
		RetryInterval:       defaultCommandRetry,
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

	s := loadSaga(t, store, factory, orderID)
	if version := s.AggregateVersion(); version != 0 {
		t.Fatalf("expected saga to remain untouched; got version %d", version)
	}
}

func TestCommandRecovery_RedispatchesSameCommandIDAfterSaveFailure(t *testing.T) {
	reg := newRegistry()
	underlying := eventstore.New()
	store := &failInsertStore{
		Store:     underlying,
		failName:  saga.CommandDispatched,
		remaining: 1,
	}
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	_, def := newOrderSagaDefinition(time.Second)
	orderID := uuid.New()

	svc := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             eventbus.New(),
		Commands:        cmdBus,
		RetryInterval:   time.Second,
		TimerResolution: defaultTimerResolution,
	}, def)
	if err := svc.Inject(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("inject placed event: %v", err)
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

	restarted := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           underlying,
		Bus:             eventbus.New(),
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
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

func TestTimeoutRecovery_FiresAfterRestart(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newOrderSagaDefinition(20 * time.Millisecond)
	orderID := uuid.New()

	svc := saga.NewService(saga.Config{
		Encoding: reg,
		Store:    store,
		Commands: cmdBus,
	}, def)
	if err := svc.Inject(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("inject placed event: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	restarted := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             eventbus.New(),
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitState(t, func() bool {
		s := loadSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusFailed && s.TimedOutCount == 1
	})

	if got := countSagaEvents(t, store, orderID, saga.TimeoutFired); got != 1 {
		t.Fatalf("expected 1 timeout fired event; got %d", got)
	}
}

func TestCompensationLifecycleAndPhaseRouting(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	bus := eventbus.New()
	cmdBus := newCommandBus(t, reg)
	defer cmdBus.Close(t)
	factory, def := newCompensationSagaDefinition(time.Second)

	svc := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             bus,
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
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
		s := loadCompensationSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusRunning && s.PlacedCount == 1
	})

	if err := bus.Publish(context.Background(), newStockReleased(orderID, uuid.New())); err != nil {
		t.Fatalf("publish forward stock released event: %v", err)
	}

	awaitState(t, func() bool {
		s := loadCompensationSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusRunning && s.ForwardReleasedCount == 1 && s.CompReleasedCount == 0
	})

	if err := bus.Publish(context.Background(), newPaymentDeclined(orderID, uuid.New())); err != nil {
		t.Fatalf("publish payment declined event: %v", err)
	}

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		s := loadCompensationSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusCompensating &&
			s.DeclinedCount == 1 &&
			s.FailureReason() == "payment declined" &&
			s.Compensation().Active
	})

	if got := countCompensationSagaEvents(t, store, orderID, saga.CompensationStarted); got != 1 {
		t.Fatalf("expected 1 compensation started event; got %d", got)
	}

	if got := countCompensationSagaEvents(t, store, orderID, saga.TimeoutCanceled); got != 1 {
		t.Fatalf("expected 1 timeout canceled event after entering compensation; got %d", got)
	}

	if err := bus.Publish(context.Background(), newOrderConfirmed(orderID, uuid.New())); err != nil {
		t.Fatalf("publish ignored confirmed event: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := bus.Publish(context.Background(), newStockReleased(orderID, uuid.New())); err != nil {
		t.Fatalf("publish compensating stock released event: %v", err)
	}

	awaitState(t, func() bool {
		s := loadCompensationSaga(t, store, factory, orderID)
		comp := s.Compensation()
		return s.Status() == saga.StatusFailed &&
			s.ForwardReleasedCount == 1 &&
			s.CompReleasedCount == 1 &&
			comp.Completed &&
			!comp.Active &&
			s.FailureReason() == "payment declined"
	})

	if got := countCompensationSagaEvents(t, store, orderID, saga.CompensationCompleted); got != 1 {
		t.Fatalf("expected 1 compensation completed event; got %d", got)
	}

	if got := countCompensationSagaEvents(t, store, orderID, saga.TimeoutCanceled); got != 2 {
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
	factory, def := newCompensationSagaDefinition(time.Second)

	svc := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             bus,
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
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
		s := loadCompensationSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusRunning
	})

	triggerID := uuid.New()
	evt := newPaymentDeclined(orderID, triggerID)
	if err := bus.Publish(context.Background(), evt, evt); err != nil {
		t.Fatalf("publish duplicate declined event: %v", err)
	}

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		s := loadCompensationSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusCompensating && s.DeclinedCount == 1
	})

	if got := countCompensationSagaEvents(t, store, orderID, saga.CompensationStarted); got != 1 {
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
	factory, def := newCompensationSagaDefinition(time.Second)
	orderID := uuid.New()

	svc := saga.NewService(saga.Config{
		Encoding: reg,
		Store:    store,
		Commands: cmdBus,
	}, def)
	if err := svc.Inject(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("inject placed event: %v", err)
	}
	if err := svc.Inject(context.Background(), newPaymentDeclined(orderID, uuid.New())); err != nil {
		t.Fatalf("inject declined event: %v", err)
	}

	if got := len(cmdBus.Commands()); got != 0 {
		t.Fatalf("expected no commands before runtime starts; got %d", got)
	}

	restarted := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             eventbus.New(),
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		s := loadCompensationSaga(t, store, factory, orderID)
		return s.Status() == saga.StatusCompensating &&
			s.Compensation().Active &&
			s.FailureReason() == "payment declined"
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
	factory, def := newCompensationSagaDefinition(20 * time.Millisecond)
	orderID := uuid.New()

	svc := saga.NewService(saga.Config{
		Encoding: reg,
		Store:    store,
		Commands: cmdBus,
	}, def)
	if err := svc.Inject(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("inject placed event: %v", err)
	}
	if err := svc.Inject(context.Background(), newPaymentDeclined(orderID, uuid.New())); err != nil {
		t.Fatalf("inject declined event: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	restarted := saga.NewService(saga.Config{
		Encoding:        reg,
		Store:           store,
		Bus:             eventbus.New(),
		Commands:        cmdBus,
		RetryInterval:   defaultCommandRetry,
		TimerResolution: defaultTimerResolution,
	}, def)

	cancel, wait := runService(t, restarted)
	defer func() {
		cancel()
		wait()
	}()

	awaitCommand(t, cmdBus, 1)
	awaitState(t, func() bool {
		s := loadCompensationSaga(t, store, factory, orderID)
		comp := s.Compensation()
		return s.Status() == saga.StatusFailed &&
			s.CompensationTimedOut == 1 &&
			comp.Failed &&
			comp.Error == "release-stock timeout" &&
			s.FailureReason() == "payment declined"
	})

	if got := countCompensationSagaEvents(t, store, orderID, saga.CompensationFailed); got != 1 {
		t.Fatalf("expected 1 compensation failed event; got %d", got)
	}

	if got := countCompensationSagaEvents(t, store, orderID, saga.TimeoutFired); got != 1 {
		t.Fatalf("expected 1 compensation timeout fired event; got %d", got)
	}
}

func TestCompensationTransitionValidation(t *testing.T) {
	t.Run("compensated outside compensation", func(t *testing.T) {
		reg := newRegistry()
		store := eventstore.New()

		type invalidSaga struct{ *saga.Base }
		factory := func(id uuid.UUID) *invalidSaga {
			return &invalidSaga{Base: saga.New("test.invalid_compensated", id)}
		}

		def := saga.Define(
			factory,
			saga.Starts(saga.AggregateID[orderPlacedData](), func(s *invalidSaga, ctx saga.Ctx[orderPlacedData]) error {
				return ctx.Compensated()
			}, orderPlacedEvent),
		)

		svc := saga.NewService(saga.Config{Encoding: reg, Store: store}, def)
		err := svc.Inject(context.Background(), newOrderPlaced(uuid.New(), uuid.New()))
		if !errors.Is(err, saga.ErrNotCompensating) {
			t.Fatalf("expected ErrNotCompensating; got %v", err)
		}
	})

	t.Run("fail while compensating", func(t *testing.T) {
		reg := newRegistry()
		store := eventstore.New()

		type invalidSaga struct{ *saga.Base }
		factory := func(id uuid.UUID) *invalidSaga {
			return &invalidSaga{Base: saga.New("test.invalid_transition", id)}
		}

		def := saga.Define(
			factory,
			saga.Starts(saga.AggregateID[orderPlacedData](), func(s *invalidSaga, ctx saga.Ctx[orderPlacedData]) error {
				return nil
			}, orderPlacedEvent),
			saga.Reacts(saga.AggregateID[paymentDeclinedData](), func(s *invalidSaga, ctx saga.Ctx[paymentDeclinedData]) error {
				if err := ctx.Compensate(errors.New("boom")); err != nil {
					return err
				}
				return ctx.Fail(errors.New("still boom"))
			}, paymentDeclinedEvent),
		)

		svc := saga.NewService(saga.Config{Encoding: reg, Store: store}, def)
		orderID := uuid.New()
		if err := svc.Inject(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
			t.Fatalf("inject placed event: %v", err)
		}

		err := svc.Inject(context.Background(), newPaymentDeclined(orderID, uuid.New()))
		if !errors.Is(err, saga.ErrInvalidTransition) {
			t.Fatalf("expected ErrInvalidTransition; got %v", err)
		}
	})
}

func TestRegisterEvents(t *testing.T) {
	reg := newRegistry()
	want := saga.TimeoutRequestedData{
		EffectID:  uuid.New(),
		Key:       "payment",
		TriggerID: uuid.New(),
		At:        time.Now().UTC().Truncate(0),
	}

	payload, err := reg.Marshal(want)
	if err != nil {
		t.Fatalf("marshal timeout request: %v", err)
	}

	gotAny, err := reg.Unmarshal(payload, saga.TimeoutRequested)
	if err != nil {
		t.Fatalf("unmarshal timeout request: %v", err)
	}

	got, ok := gotAny.(saga.TimeoutRequestedData)
	if !ok {
		t.Fatalf("expected saga.TimeoutRequestedData; got %T", gotAny)
	}

	if got != want {
		t.Fatalf("decoded payload mismatch: want=%#v got=%#v", want, got)
	}

	compWant := saga.CompensationStartedData{Reason: "boom"}
	compPayload, err := reg.Marshal(compWant)
	if err != nil {
		t.Fatalf("marshal compensation started: %v", err)
	}

	compAny, err := reg.Unmarshal(compPayload, saga.CompensationStarted)
	if err != nil {
		t.Fatalf("unmarshal compensation started: %v", err)
	}

	compGot, ok := compAny.(saga.CompensationStartedData)
	if !ok {
		t.Fatalf("expected saga.CompensationStartedData; got %T", compAny)
	}

	if compGot != compWant {
		t.Fatalf("decoded compensation payload mismatch: want=%#v got=%#v", compWant, compGot)
	}
}

func newRegistry() *codec.Registry {
	reg := codec.New()
	saga.RegisterEvents(reg)
	codec.Register[orderPlacedData](reg, orderPlacedEvent)
	codec.Register[orderConfirmedData](reg, orderConfirmedEvent)
	codec.Register[paymentDeclinedData](reg, paymentDeclinedEvent)
	codec.Register[stockReleasedData](reg, stockReleasedEvent)
	codec.Register[reserveStockData](reg, reserveStockCommand)
	codec.Register[releaseStockData](reg, releaseStockCommand)
	codec.Register[placedRecordedData](reg, orderPlacedRecorded)
	codec.Register[confirmedRecordedData](reg, orderConfirmedRecorded)
	codec.Register[timedOutRecordedData](reg, orderTimedOutRecorded)
	codec.Register[compensationPlacedRecordedData](reg, compSagaPlacedRecorded)
	codec.Register[compensationDeclinedRecordedData](reg, compSagaDeclinedRecorded)
	codec.Register[compensationForwardReleaseRecordedData](reg, compSagaForwardReleaseRecorded)
	codec.Register[compensationCompensatedReleaseRecordedData](reg, compSagaCompReleaseRecorded)
	codec.Register[compensationTimeoutRecordedData](reg, compSagaTimeoutRecorded)
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

func runService(t *testing.T, svc *saga.Service) (context.CancelFunc, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	errs, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("run saga service: %v", err)
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

	t.Fatal("timed out waiting for saga state")
}

func loadSaga(t *testing.T, store event.Store, factory func(uuid.UUID) *orderSaga, id uuid.UUID) *orderSaga {
	t.Helper()

	repo := aggrepo.New(store)
	s := factory(id)
	if err := repo.Fetch(context.Background(), s); err != nil {
		events, errs, qerr := store.Query(context.Background(), query.New(
			query.AggregateName(orderSagaAggregateName),
			query.AggregateID(id),
			query.SortBy(event.SortAggregateVersion, event.SortAsc),
		))
		if qerr != nil {
			t.Fatalf("fetch saga %s: %v (query debug failed: %v)", id, err, qerr)
		}
		all, derr := streams.Drain(context.Background(), events, errs)
		if derr != nil {
			t.Fatalf("fetch saga %s: %v (drain debug failed: %v)", id, err, derr)
		}
		t.Fatalf("fetch saga %s: %v (events=%v)", id, err, eventVersions(all))
	}
	return s
}

func countSagaEvents(t *testing.T, store event.Store, sagaID uuid.UUID, names ...string) int {
	t.Helper()
	return countAggregateEvents(t, store, orderSagaAggregateName, sagaID, names...)
}

func loadCompensationSaga(t *testing.T, store event.Store, factory func(uuid.UUID) *compensationSaga, id uuid.UUID) *compensationSaga {
	t.Helper()

	repo := aggrepo.New(store)
	s := factory(id)
	if err := repo.Fetch(context.Background(), s); err != nil {
		events, errs, qerr := store.Query(context.Background(), query.New(
			query.AggregateName(compSagaAggregateName),
			query.AggregateID(id),
			query.SortBy(event.SortAggregateVersion, event.SortAsc),
		))
		if qerr != nil {
			t.Fatalf("fetch compensation saga %s: %v (query debug failed: %v)", id, err, qerr)
		}
		all, derr := streams.Drain(context.Background(), events, errs)
		if derr != nil {
			t.Fatalf("fetch compensation saga %s: %v (drain debug failed: %v)", id, err, derr)
		}
		t.Fatalf("fetch compensation saga %s: %v (events=%v)", id, err, eventVersions(all))
	}
	return s
}

func countCompensationSagaEvents(t *testing.T, store event.Store, sagaID uuid.UUID, names ...string) int {
	t.Helper()
	return countAggregateEvents(t, store, compSagaAggregateName, sagaID, names...)
}

func countAggregateEvents(t *testing.T, store event.Store, aggregateName string, sagaID uuid.UUID, names ...string) int {
	t.Helper()
	events, errs, err := store.Query(context.Background(), query.New(
		query.Name(names...),
		query.AggregateName(aggregateName),
		query.AggregateID(sagaID),
		query.SortByTime(),
	))
	if err != nil {
		t.Fatalf("query saga events %v: %v", names, err)
	}

	all, err := streams.Drain(context.Background(), events, errs)
	if err != nil {
		t.Fatalf("drain saga events %v: %v", names, err)
	}

	return len(all)
}

func eventVersions(events []event.Event) []string {
	out := make([]string, 0, len(events))
	for _, evt := range events {
		_, _, version := evt.Aggregate()
		out = append(out, fmt.Sprintf("%s@%d", evt.Name(), version))
	}
	return out
}
