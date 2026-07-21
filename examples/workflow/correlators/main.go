// Command correlators demonstrates the different ways trigger events are
// correlated to workflow instances:
//
//   - workflow.ByAggregateID: the workflow shares its id with the aggregate
//     that emitted the trigger (see the "basic" example)
//   - workflow.ByKey: the workflow id is derived from a business key in the
//     event payload (here: one loyalty workflow per customer email),
//     collecting events from many different aggregates into one workflow
//   - hand-written correlators: a Correlator is just a function, so any
//     custom logic works — including returning false to ignore events
//     entirely (here: orders below a minimum amount)
//   - one handler registered for multiple event names that share a payload
//     type (here: shipping updates)
//
// It also shows that a Starts handler advances an existing workflow like a
// Reacts handler (only the first order creates the workflow), and what
// Config.Strict does with events that correlate to unknown workflows.
package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/workflow"
)

// Domain events. Every order is its own aggregate; the loyalty workflow is
// keyed by customer email instead.
const (
	OrderAggregate = "shop.order"

	OrderPlaced    = "shop.order.placed"
	OrderShipped   = "shop.order.shipped"
	OrderDelivered = "shop.order.delivered"
)

type OrderPlacedData struct {
	CustomerEmail string
	Amount        int
}

// ShippingUpdateData is shared by OrderShipped and OrderDelivered, which
// allows a single trigger registration to handle both events.
type ShippingUpdateData struct {
	CustomerEmail string
}

// LoyaltyWorkflow counts the qualified orders of a customer and completes
// once the customer reaches gold status.
type LoyaltyWorkflow struct {
	*workflow.Base

	Orders int
}

const (
	LoyaltyWorkflowName = "loyalty.workflow"

	// OrderCounted is an event of the workflow itself, persisted alongside
	// the built-in workflow events.
	OrderCounted = "loyalty.workflow.order_counted"

	goldStatusAt = 3
	minAmount    = 10
)

type OrderCountedData struct {
	Amount int
}

// customerNamespace derives deterministic workflow ids from customer emails,
// so that all events of a customer correlate to the same workflow instance.
var customerNamespace = uuid.MustParse("d3f0a1de-52a7-40e8-8a3a-79dbd8f2071e")

func customerWorkflowID(email string) uuid.UUID {
	return uuid.NewSHA1(customerNamespace, []byte(email))
}

func NewLoyaltyWorkflow(id uuid.UUID) *LoyaltyWorkflow {
	w := &LoyaltyWorkflow{Base: workflow.New(LoyaltyWorkflowName, id)}
	event.ApplyWith(w, w.orderCounted, OrderCounted)
	return w
}

func (w *LoyaltyWorkflow) orderCounted(event.Of[OrderCountedData]) {
	w.Orders++
}

// byOrder is a hand-written correlator: it correlates orders to the
// customer's loyalty workflow and ignores orders below the minimum amount by
// returning false. It derives the same workflow ids as workflow.ByKey below,
// because both use UUIDv5 derivation within the same namespace.
func byOrder(evt event.Of[OrderPlacedData]) (uuid.UUID, bool) {
	if evt.Data().Amount < minAmount {
		return uuid.Nil, false
	}
	return customerWorkflowID(evt.Data().CustomerEmail), true
}

var LoyaltyProgram = workflow.Define(
	NewLoyaltyWorkflow,

	// The first qualified order starts the workflow; further orders advance
	// it through the same handler.
	workflow.Starts(byOrder, (*LoyaltyWorkflow).onOrder, OrderPlaced),

	// The built-in ByKey correlator derives the workflow id from a business
	// key in the payload. One registration for two event names.
	workflow.Reacts(
		workflow.ByKey(customerNamespace, func(d ShippingUpdateData) string { return d.CustomerEmail }),
		(*LoyaltyWorkflow).onShippingUpdate,
		OrderShipped, OrderDelivered,
	),
)

func (w *LoyaltyWorkflow) onOrder(ctx workflow.Ctx[OrderPlacedData]) error {
	aggregate.Next(w, OrderCounted, OrderCountedData{Amount: ctx.Event().Data().Amount})

	log.Printf("[loyalty ] %s: order #%d counted ($%d)", ctx.Event().Data().CustomerEmail, w.Orders, ctx.Event().Data().Amount)

	if w.Orders >= goldStatusAt {
		log.Printf("[loyalty ] %s: gold status awarded 🏆", ctx.Event().Data().CustomerEmail)
		return ctx.Complete()
	}

	return nil
}

func (w *LoyaltyWorkflow) onShippingUpdate(ctx workflow.Ctx[ShippingUpdateData]) error {
	log.Printf("[loyalty ] %s: shipping update %q", ctx.Event().Data().CustomerEmail, ctx.Event().Name())
	return nil
}

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := codec.New()
	codec.Register[OrderPlacedData](reg, OrderPlaced)
	codec.Register[ShippingUpdateData](reg, OrderShipped)
	codec.Register[ShippingUpdateData](reg, OrderDelivered)
	codec.Register[OrderCountedData](reg, OrderCounted)

	ebus := eventbus.New()
	store := eventstore.New()

	svc := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
		EventBus:   ebus,
	}, LoyaltyProgram)

	errs, err := svc.Run(ctx)
	must(err)
	go drain("workflow service", errs)

	repo := repository.New(store)
	alice := customerWorkflowID("alice@example.com")
	bob := customerWorkflowID("bob@example.com")

	// A $5 order does not correlate to any workflow: the correlator returns
	// false and the event is ignored entirely.
	placeOrder(ctx, ebus, "alice@example.com", 5)
	time.Sleep(100 * time.Millisecond)
	log.Printf("[shop    ] $5 order ignored; alice has no loyalty workflow yet (version %d)", version(repo, alice))

	// Qualified orders start and advance alice's workflow. Shipping updates
	// from different order aggregates correlate into the same workflow.
	placeOrder(ctx, ebus, "alice@example.com", 25)
	shippingUpdate(ctx, ebus, OrderShipped, "alice@example.com")
	placeOrder(ctx, ebus, "alice@example.com", 40)
	shippingUpdate(ctx, ebus, OrderDelivered, "alice@example.com")
	placeOrder(ctx, ebus, "alice@example.com", 15)

	// Bob gets his own workflow, derived from his email.
	placeOrder(ctx, ebus, "bob@example.com", 30)

	await("alice to reach gold status", func() bool {
		return fetch(repo, alice).Status() == workflow.StatusCompleted
	})
	await("bob's workflow to start", func() bool {
		return fetch(repo, bob).Status() == workflow.StatusRunning
	})

	aliceWF, bobWF := fetch(repo, alice), fetch(repo, bob)
	log.Printf("alice: status=%s orders=%d", aliceWF.Status(), aliceWF.Orders)
	log.Printf("bob:   status=%s orders=%d", bobWF.Status(), bobWF.Orders)

	// Strict mode: events that correlate to an unknown workflow are reported
	// as errors instead of being silently ignored (a shipping update for a
	// customer without a workflow, in this case).
	log.Printf("--- strict mode ---")

	strictBus := eventbus.New()
	strict := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: eventstore.New(),
		EventBus:   strictBus,
		Strict:     true,
	}, LoyaltyProgram)

	strictErrs, err := strict.Run(ctx)
	must(err)

	shippingUpdate(ctx, strictBus, OrderShipped, "carol@example.com")

	select {
	case err := <-strictErrs:
		log.Printf("strict service reported: %v", err)
	case <-time.After(5 * time.Second):
		log.Fatal("timed out waiting for strict-mode error")
	}
}

func placeOrder(ctx context.Context, bus event.Bus, email string, amount int) {
	must(bus.Publish(ctx, event.New(
		OrderPlaced,
		OrderPlacedData{CustomerEmail: email, Amount: amount},
		event.Aggregate(uuid.New(), OrderAggregate, 1),
	).Any()))
}

func shippingUpdate(ctx context.Context, bus event.Bus, name, email string) {
	must(bus.Publish(ctx, event.New(
		name,
		ShippingUpdateData{CustomerEmail: email},
		event.Aggregate(uuid.New(), OrderAggregate, 2),
	).Any()))
}

func fetch(repo *repository.Repository, id uuid.UUID) *LoyaltyWorkflow {
	w := NewLoyaltyWorkflow(id)
	must(repo.Fetch(context.Background(), w))
	return w
}

func version(repo *repository.Repository, id uuid.UUID) int {
	return fetch(repo, id).AggregateVersion()
}

func await(what string, cond func() bool) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	log.Fatalf("timed out waiting for %s", what)
}

func drain(name string, errs <-chan error) {
	for err := range errs {
		if err != nil {
			log.Printf("[%s] error: %v", name, err)
		}
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
