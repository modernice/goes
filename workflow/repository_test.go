package workflow_test

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	aggrepo "github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/workflow"
)

// countingRepository wraps an aggregate.Repository and counts fetches and
// saves.
type countingRepository struct {
	aggregate.Repository

	fetches atomic.Int32
	saves   atomic.Int32
}

func (r *countingRepository) Fetch(ctx context.Context, a aggregate.Aggregate) error {
	r.fetches.Add(1)
	return r.Repository.Fetch(ctx, a)
}

func (r *countingRepository) Save(ctx context.Context, a aggregate.Aggregate) error {
	r.saves.Add(1)
	return r.Repository.Save(ctx, a)
}

func TestWithRepository(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	counting := &countingRepository{Repository: aggrepo.New(store)}
	factory, _ := newOrderWorkflowDefinition(time.Hour)

	def := workflow.Define(
		factory,
		workflow.WithRepository(aggrepo.Typed(counting, factory)),
		workflow.Starts(workflow.ByAggregateID, (*orderWorkflow).onPlaced, orderPlacedEvent),
	)

	svc := workflow.NewService(workflow.Config{Commands: reg, EventStore: store}, def)

	orderID := uuid.New()
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	if got := counting.fetches.Load(); got == 0 {
		t.Fatal("expected the definition's repository to be used for fetching")
	}
	if got := counting.saves.Load(); got == 0 {
		t.Fatal("expected the definition's repository to be used for saving")
	}

	w := loadWorkflow(t, store, factory, orderID)
	if w.Status() != workflow.StatusRunning {
		t.Fatalf("expected workflow to be running; got %q", w.Status())
	}
}

func TestConfig_NewRepository(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	counting := &countingRepository{Repository: aggrepo.New(store)}
	factory, def := newOrderWorkflowDefinition(time.Hour)

	svc := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
		NewRepository: func(event.Store) aggregate.Repository {
			return counting
		},
	}, def)

	orderID := uuid.New()
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}

	if counting.fetches.Load() == 0 || counting.saves.Load() == 0 {
		t.Fatal("expected the configured repository to be used")
	}

	w := loadWorkflow(t, store, factory, orderID)
	if w.Status() != workflow.StatusRunning {
		t.Fatalf("expected workflow to be running; got %q", w.Status())
	}
}

func TestWithRepository_Snapshots(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	snapshots := snapshot.NewStore()

	// snapWorkflow carries no state of its own, so the promoted snapshot
	// marshaling of workflow.Base captures its full state.
	type snapWorkflow struct{ *workflow.Base }
	factory := func(id uuid.UUID) *snapWorkflow {
		return &snapWorkflow{Base: workflow.New("test.snap_workflow", id)}
	}

	repo := aggrepo.New(store, aggrepo.WithSnapshots(snapshots, snapshot.Every(1)))
	typed := aggrepo.Typed(repo, factory)

	def := workflow.Define(
		factory,
		workflow.WithRepository(typed),
		workflow.Starts(workflow.ByAggregateID, func(w *snapWorkflow, ctx workflow.Ctx[orderPlacedData]) error {
			return ctx.Schedule("payment", time.Now().Add(time.Hour))
		}, orderPlacedEvent),
		workflow.Reacts(workflow.ByAggregateID, func(w *snapWorkflow, ctx workflow.Ctx[orderConfirmedData]) error {
			if err := ctx.Unschedule("payment"); err != nil {
				return err
			}
			return ctx.Complete()
		}, orderConfirmedEvent),
	)

	svc := workflow.NewService(workflow.Config{Commands: reg, EventStore: store}, def)

	orderID := uuid.New()
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}
	if err := svc.Trigger(context.Background(), newOrderConfirmed(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger confirmed event: %v", err)
	}

	if _, err := snapshots.Latest(context.Background(), "test.snap_workflow", orderID); err != nil {
		t.Fatalf("expected a snapshot to have been taken: %v", err)
	}

	// The snapshot-fetched workflow must equal the purely replayed one.
	fromSnapshot, err := typed.Fetch(context.Background(), orderID)
	if err != nil {
		t.Fatalf("fetch workflow via snapshot repository: %v", err)
	}
	if fromSnapshot.Status() != workflow.StatusCompleted {
		t.Fatalf("expected snapshot-fetched workflow to be completed; got %q", fromSnapshot.Status())
	}

	replayed := factory(orderID)
	if err := aggrepo.New(store).Fetch(context.Background(), replayed); err != nil {
		t.Fatalf("fetch workflow via event replay: %v", err)
	}

	snapState, err := fromSnapshot.MarshalSnapshot()
	if err != nil {
		t.Fatalf("marshal snapshot-fetched workflow: %v", err)
	}
	replayState, err := replayed.MarshalSnapshot()
	if err != nil {
		t.Fatalf("marshal replayed workflow: %v", err)
	}

	if !bytes.Equal(snapState, replayState) {
		t.Fatal("snapshot-fetched workflow state differs from replayed workflow state")
	}
}

func TestBaseSnapshot_Roundtrip(t *testing.T) {
	reg := newRegistry()
	store := eventstore.New()
	factory, def := newCompWorkflowDefinition(time.Hour)

	svc := workflow.NewService(workflow.Config{Commands: reg, EventStore: store}, def)

	orderID := uuid.New()
	if err := svc.Trigger(context.Background(), newOrderPlaced(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger placed event: %v", err)
	}
	if err := svc.Trigger(context.Background(), newPaymentDeclined(orderID, uuid.New())); err != nil {
		t.Fatalf("trigger declined event: %v", err)
	}

	original := loadWorkflow(t, store, factory, orderID)
	state, err := original.MarshalSnapshot()
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	restored := factory(orderID)
	if err := restored.UnmarshalSnapshot(state); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	if restored.Status() != original.Status() || restored.Reason() != original.Reason() {
		t.Fatalf(
			"restored state mismatch: status %q/%q, reason %q/%q",
			restored.Status(), original.Status(), restored.Reason(), original.Reason(),
		)
	}

	restoredState, err := restored.MarshalSnapshot()
	if err != nil {
		t.Fatalf("marshal restored snapshot: %v", err)
	}
	if !bytes.Equal(state, restoredState) {
		t.Fatal("restored snapshot state differs from original")
	}
}
