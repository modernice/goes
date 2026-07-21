// Command timeouts demonstrates everything around workflow timeouts:
//
//   - multiple concurrent timeouts per workflow (a reminder and a deadline)
//   - a timeout handler that leaves the workflow running (the reminder)
//   - rescheduling an active timeout by calling Ctx.Schedule with the same
//     key again (extending the deadline)
//   - canceling timeouts and completing (payment received)
//   - a timeout handler that fails the workflow (the deadline)
//
// Invoice A gets extended and paid: its reminder fires, its deadline never
// does. Invoice B is never paid: its reminder fires, then its deadline fires
// and fails the workflow.
package main

import (
	"context"
	"errors"
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

const (
	InvoiceAggregate = "billing.invoice"

	InvoiceIssued     = "billing.invoice.issued"
	ExtensionGranted  = "billing.invoice.extension_granted"
	InvoicePaid       = "billing.invoice.paid"
	ReminderSent      = "billing.invoice_workflow.reminder_sent"
	InvoiceWorkflowNm = "billing.invoice_workflow"

	reminderAfter = 300 * time.Millisecond
	dueAfter      = 700 * time.Millisecond
	extendBy      = 900 * time.Millisecond
)

type InvoiceIssuedData struct {
	Amount int
}

type ExtensionGrantedData struct{}

type InvoicePaidData struct{}

type ReminderSentData struct{}

// InvoiceWorkflow chases an invoice until it is paid or overdue.
type InvoiceWorkflow struct {
	*workflow.Base

	Reminders int
}

func NewInvoiceWorkflow(id uuid.UUID) *InvoiceWorkflow {
	w := &InvoiceWorkflow{Base: workflow.New(InvoiceWorkflowNm, id)}
	event.ApplyWith(w, w.reminderSent, ReminderSent)
	return w
}

func (w *InvoiceWorkflow) reminderSent(event.Of[ReminderSentData]) {
	w.Reminders++
}

var InvoiceProcess = workflow.Define(
	NewInvoiceWorkflow,
	workflow.Starts(workflow.ByAggregateID, (*InvoiceWorkflow).onIssued, InvoiceIssued),
	workflow.Reacts(workflow.ByAggregateID, (*InvoiceWorkflow).onExtension, ExtensionGranted),
	workflow.Reacts(workflow.ByAggregateID, (*InvoiceWorkflow).onPaid, InvoicePaid),
	workflow.OnTimeout("reminder", (*InvoiceWorkflow).onReminder),
	workflow.OnTimeout("deadline", (*InvoiceWorkflow).onDeadline),
)

func (w *InvoiceWorkflow) onIssued(ctx workflow.Ctx[InvoiceIssuedData]) error {
	log.Printf("[workflow] invoice issued ($%d); scheduling reminder and deadline", ctx.Event().Data().Amount)

	// Two independent timeouts, identified by their keys.
	if err := ctx.Schedule("reminder", time.Now().Add(reminderAfter)); err != nil {
		return err
	}
	return ctx.Schedule("deadline", time.Now().Add(dueAfter))
}

func (w *InvoiceWorkflow) onExtension(ctx workflow.Ctx[ExtensionGrantedData]) error {
	log.Printf("[workflow] extension granted; moving the deadline")

	// Scheduling an already active key replaces the timeout.
	return ctx.Schedule("deadline", time.Now().Add(extendBy))
}

func (w *InvoiceWorkflow) onPaid(ctx workflow.Ctx[InvoicePaidData]) error {
	log.Printf("[workflow] invoice paid; completing")

	// Complete cancels all remaining timeouts, but timeouts can also be
	// canceled individually:
	if err := ctx.Unschedule("deadline"); err != nil {
		return err
	}

	return ctx.Complete()
}

func (w *InvoiceWorkflow) onReminder(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	// A timeout handler that records progress and leaves the workflow
	// running.
	aggregate.Next(w, ReminderSent, ReminderSentData{})
	log.Printf("[workflow] reminder sent (scheduled for %s)", ctx.Event().Data().ScheduledFor.Format(time.TimeOnly))
	return nil
}

func (w *InvoiceWorkflow) onDeadline(ctx workflow.Ctx[workflow.TimeoutFiredData]) error {
	log.Printf("[workflow] deadline passed; failing")
	return ctx.Fail(errors.New("invoice overdue"))
}

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := codec.New()
	codec.Register[InvoiceIssuedData](reg, InvoiceIssued)
	codec.Register[ExtensionGrantedData](reg, ExtensionGranted)
	codec.Register[InvoicePaidData](reg, InvoicePaid)
	codec.Register[ReminderSentData](reg, ReminderSent)

	ebus := eventbus.New()
	store := eventstore.New()

	svc := workflow.NewService(workflow.Config{
		Commands:        reg,
		EventStore:      store,
		EventBus:        ebus,
		TimerResolution: 10 * time.Millisecond,
	}, InvoiceProcess)

	errs, err := svc.Run(ctx)
	must(err)
	go drain("workflow service", errs)

	repo := repository.New(store)

	invoiceA := uuid.New()
	invoiceB := uuid.New()

	log.Printf("[billing ] issuing invoice A (%s) and invoice B (%s)", invoiceA, invoiceB)
	issue(ctx, ebus, invoiceA)
	issue(ctx, ebus, invoiceB)

	// Invoice A: extend the deadline shortly after issuing, pay before the
	// extended deadline expires.
	time.Sleep(100 * time.Millisecond)
	must(ebus.Publish(ctx, event.New(
		ExtensionGranted, ExtensionGrantedData{}, event.Aggregate(invoiceA, InvoiceAggregate, 2),
	).Any()))

	// Wait until the original deadline would have fired; only the reminders
	// fire in the meantime.
	time.Sleep(dueAfter)
	must(ebus.Publish(ctx, event.New(
		InvoicePaid, InvoicePaidData{}, event.Aggregate(invoiceA, InvoiceAggregate, 3),
	).Any()))

	await("invoice A to complete", func() bool {
		return fetch(repo, invoiceA).Status() == workflow.StatusCompleted
	})

	// Invoice B is never paid: its deadline fires and fails the workflow.
	await("invoice B to fail", func() bool {
		return fetch(repo, invoiceB).Status() == workflow.StatusFailed
	})

	a, b := fetch(repo, invoiceA), fetch(repo, invoiceB)
	log.Printf("invoice A: status=%s reminders=%d", a.Status(), a.Reminders)
	log.Printf("invoice B: status=%s reminders=%d reason=%q", b.Status(), b.Reminders, b.Reason())
}

func issue(ctx context.Context, bus event.Bus, invoiceID uuid.UUID) {
	must(bus.Publish(ctx, event.New(
		InvoiceIssued,
		InvoiceIssuedData{Amount: 100},
		event.Aggregate(invoiceID, InvoiceAggregate, 1),
	).Any()))
}

func fetch(repo *repository.Repository, id uuid.UUID) *InvoiceWorkflow {
	w := NewInvoiceWorkflow(id)
	must(repo.Fetch(context.Background(), w))
	return w
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
