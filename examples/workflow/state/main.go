// Command state demonstrates workflows with their own event-sourced state.
//
// Workflows are regular aggregates: handlers record custom events with
// aggregate.Next, appliers registered via event.ApplyWith rebuild the state,
// and decisions are made from that state. Here, a document review needs two
// approvals from distinct reviewers before the document gets published:
//
//   - each approval is recorded as an ApprovalRecorded event of the workflow
//   - a repeated approval by the same reviewer records nothing
//     (business-level idempotency on top of the built-in trigger dedup)
//   - the second approval dispatches a publish command and completes
//   - a rejection fails the workflow instead
package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/repository"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/command"
	"github.com/modernice/goes/command/cmdbus"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/eventbus"
	"github.com/modernice/goes/event/eventstore"
	"github.com/modernice/goes/workflow"
)

const (
	DocumentAggregate = "docs.document"

	ReviewRequested = "docs.document.review_requested"
	Approved        = "docs.document.approved"
	Rejected        = "docs.document.rejected"

	PublishDocument = "docs.publish_document"

	ReviewWorkflowName = "docs.review_workflow"

	// ApprovalRecorded is an event of the workflow itself.
	ApprovalRecorded = "docs.review_workflow.approval_recorded"

	requiredApprovals = 2
)

type ReviewRequestedData struct{ Title string }
type ApprovedData struct{ Reviewer string }
type RejectedData struct {
	Reviewer string
	Reason   string
}

type ApprovalRecordedData struct{ Reviewer string }

type PublishPayload struct{ DocumentID uuid.UUID }

// ReviewWorkflow collects approvals for a document until enough distinct
// reviewers approved it.
type ReviewWorkflow struct {
	*workflow.Base

	approvals map[string]bool
}

func NewReviewWorkflow(id uuid.UUID) *ReviewWorkflow {
	w := &ReviewWorkflow{
		Base:      workflow.New(ReviewWorkflowName, id),
		approvals: make(map[string]bool),
	}

	event.ApplyWith(w, w.approvalRecorded, ApprovalRecorded)

	return w
}

func (w *ReviewWorkflow) approvalRecorded(evt event.Of[ApprovalRecordedData]) {
	w.approvals[evt.Data().Reviewer] = true
}

// Approvers returns the reviewers that approved the document so far.
func (w *ReviewWorkflow) Approvers() []string {
	approvers := make([]string, 0, len(w.approvals))
	for reviewer := range w.approvals {
		approvers = append(approvers, reviewer)
	}
	sort.Strings(approvers)
	return approvers
}

var ReviewProcess = workflow.Define(
	NewReviewWorkflow,
	workflow.Starts(workflow.ByAggregateID, (*ReviewWorkflow).onRequested, ReviewRequested),
	workflow.Reacts(workflow.ByAggregateID, (*ReviewWorkflow).onApproved, Approved),
	workflow.Reacts(workflow.ByAggregateID, (*ReviewWorkflow).onRejected, Rejected),
)

func (w *ReviewWorkflow) onRequested(ctx workflow.Ctx[ReviewRequestedData]) error {
	log.Printf("[workflow] review of %q requested (%d approvals needed)", ctx.Event().Data().Title, requiredApprovals)
	return nil
}

func (w *ReviewWorkflow) onApproved(ctx workflow.Ctx[ApprovedData]) error {
	reviewer := ctx.Event().Data().Reviewer

	if w.approvals[reviewer] {
		log.Printf("[workflow] %s already approved; nothing to record", reviewer)
		return nil
	}

	// Record the approval as an event of the workflow. The applier updates
	// w.approvals immediately, so the state below is already current.
	aggregate.Next(w, ApprovalRecorded, ApprovalRecordedData{Reviewer: reviewer})
	log.Printf("[workflow] approval %d/%d by %s", len(w.approvals), requiredApprovals, reviewer)

	if len(w.approvals) < requiredApprovals {
		return nil
	}

	docID, _, _ := ctx.Event().Aggregate()
	if err := ctx.Dispatch("publish", command.New(
		PublishDocument, PublishPayload{DocumentID: docID}, command.Aggregate(DocumentAggregate, docID),
	).Any()); err != nil {
		return err
	}

	return ctx.Complete()
}

func (w *ReviewWorkflow) onRejected(ctx workflow.Ctx[RejectedData]) error {
	data := ctx.Event().Data()
	return ctx.Fail(fmt.Errorf("rejected by %s: %s", data.Reviewer, data.Reason))
}

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := codec.New()
	codec.Register[ReviewRequestedData](reg, ReviewRequested)
	codec.Register[ApprovedData](reg, Approved)
	codec.Register[RejectedData](reg, Rejected)
	codec.Register[ApprovalRecordedData](reg, ApprovalRecorded)
	codec.Register[PublishPayload](reg, PublishDocument)

	ebus := eventbus.New()
	store := eventstore.New()

	cmdBus := cmdbus.New[int](reg, ebus)
	cmdBusErrs, err := cmdBus.Run(ctx)
	must(err)
	go drain("command bus", cmdBusErrs)

	go drain("publish handler", command.MustHandle(ctx, cmdBus, PublishDocument, func(cmd command.Ctx[PublishPayload]) error {
		log.Printf("[docs    ] publishing document %s", cmd.Payload().DocumentID)
		return nil
	}))

	svc := workflow.NewService(workflow.Config{
		Commands:   reg,
		EventStore: store,
		EventBus:   ebus,
		CommandBus: cmdBus,
	}, ReviewProcess)

	svcErrs, err := svc.Run(ctx)
	must(err)
	go drain("workflow service", svcErrs)

	docID := uuid.New()

	must(ebus.Publish(ctx, event.New(
		ReviewRequested, ReviewRequestedData{Title: "Q3 Report"}, event.Aggregate(docID, DocumentAggregate, 1),
	).Any()))

	approve := func(reviewer string, version int) {
		must(ebus.Publish(ctx, event.New(
			Approved, ApprovedData{Reviewer: reviewer}, event.Aggregate(docID, DocumentAggregate, version),
		).Any()))
	}

	approve("alice", 2)
	approve("alice", 3) // Same reviewer again: recorded triggers, no new approval.
	approve("bob", 4)   // Second distinct reviewer: publish + complete.

	repo := repository.New(store)
	await("review to complete", func() bool {
		w := NewReviewWorkflow(docID)
		must(repo.Fetch(context.Background(), w))
		return w.Status() == workflow.StatusCompleted
	})

	// Rehydrate the workflow from the event store: the approval state is
	// rebuilt from the recorded ApprovalRecorded events.
	w := NewReviewWorkflow(docID)
	must(repo.Fetch(context.Background(), w))
	log.Printf("final status: %s (approved by %v)", w.Status(), w.Approvers())
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
