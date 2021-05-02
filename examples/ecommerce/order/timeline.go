package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/project"
)

var (
	// ErrTimelineNotFound is returned when a Timeline doesn't exist in a
	// TimelineRepository.
	ErrTimelineNotFound = errors.New("Timeline not found")
)

// TimelineRepository is the repository for Timelines.
type TimelineRepository interface {
	Save(context.Context, *Timeline) error
	Fetch(context.Context, uuid.UUID) (*Timeline, error)
}

// A Timeline is a projection of an Order that lists the steps of an Order as a timeline.
type Timeline struct {
	// Embedding a *project.Projection is not necessary, but enables a
	// projection to be applied continuously because *project.Projection
	// implements hooks that give the projector information on when the last
	// Event has been applied to the projection etc.
	*project.Projection `bson:"projection"`

	ID         uuid.UUID `bson:"id"`
	PlacedAt   time.Time `bson:"placedAt"`
	CanceledAt time.Time `bson:"canceledAt"`
	Steps      []Step    `bson:"steps"`
}

// A Step is an item of a Timeline.
type Step struct {
	Start time.Time `bson:"start"`
	End   time.Time `bson:"end"`
	Desc  string    `bson:"desc"`
}

// TimelineProjector is the projector for Timelines.
type TimelineProjector struct {
	repo     TimelineRepository
	schedule project.Schedule
}

// NewTimeline returns the Timeline for the Order with the given UUID.
func NewTimeline(id uuid.UUID) *Timeline {
	return &Timeline{
		Projection: project.NewProjection(),
		ID:         id,
	}
}

// Duration returns the total lifetime of an Order.
func (tl *Timeline) Duration() (time.Duration, bool) {
	if tl.CanceledAt.IsZero() {
		return 0, false
	}
	return tl.CanceledAt.Sub(tl.PlacedAt), true
}

// Guard guards the projection from Events. Guard receives Events before they
// are applied to the Timeline and determines if the Event should be applied.
//
// Here we are validating that the only apply Event of the "Order" Aggregate
// and from those Events only those that belong to the Order with the UUID equal
// to the UUID from the Timeline.
//
// A projection does not have to implement Guard, only ApplyEvent.
func (tl *Timeline) Guard(evt event.Event) bool {
	switch evt.AggregateName() {
	case AggregateName: // "order"
		return evt.AggregateID() == tl.ID
	default:
		return false
	}
}

// ApplyEvent applies an Event onto the Timeline (projection).
func (tl *Timeline) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case Placed:
		tl.placed(evt)
	case Canceled:
		tl.canceled(evt)
	}
	tl.finalizeSteps()
}

// placed applies the "order.placed" Event.
func (tl *Timeline) placed(evt event.Event) {
	data := evt.Data().(PlacedEvent)
	tl.PlacedAt = evt.Time()
	tl.Steps = append(tl.Steps, Step{
		Start: evt.Time(),
		Desc:  fmt.Sprintf("%s placed an Order for %d Items at %s.", data.Customer.Name, len(data.Items), evt.Time()),
	})
}

// canceled applies the "order.canceled" Event.
func (tl *Timeline) canceled(evt event.Event) {
	tl.CanceledAt = evt.Time()
	tl.Steps = append(tl.Steps, Step{
		Start: evt.Time(),
		Desc:  fmt.Sprintf("Order was canceled at %s.", evt.Time()),
	})
}

// finalizeSteps sets the Start and End times of the Timelines Steps.
func (tl *Timeline) finalizeSteps() {
	for i, step := range tl.Steps {
		if i == 0 {
			continue
		}
		tl.Steps[i-1].End = step.Start
	}
}

// NewTimelineProjector returns a TimelineProjector.
func NewTimelineProjector(bus event.Bus, store event.Store, repo TimelineRepository) *TimelineProjector {
	// Here we create a projection Schedule. This Schedule triggers projections
	// on every Event with a name that we pass here (Events[:]). The Schedule
	// debounces Events by 100ms, so that if e.g. 100 Events are received in a
	// short period of time, only 1 projection Job is created with those 100
	// Events instead of 100 Jobs with 1 Event each.
	s := project.Continuously(bus, store, Events[:], project.Debounce(100*time.Millisecond))

	return &TimelineProjector{
		repo:     repo,
		schedule: s,
	}
}

// Run starts the TimelineProjector in the background and returns a channel of
// projection errors. Callers must receive from the error channel.
func (p *TimelineProjector) Run(ctx context.Context) (<-chan error, error) {
	return p.schedule.Subscribe(ctx, p.applyJob)
}

// All projects the Timeline for every Order. This works by triggering the
// projection Schedule that was subscribed to by a call to Run so that a
// projection Job is created and passed to p.applyJob.
//
// All must not be called before Run has been called. Otherwise the projection
// Job won't be executed by p.applyJob, because it hasn't been subscribed to the
// Schedule yet.
func (p *TimelineProjector) All(ctx context.Context) error {
	return p.schedule.Trigger(ctx)
}

// Project projects the Timeline for the Order with the given UUID. This works
// by triggering the projection Schedule with an additional Event Query that
// only allows Events of "order" Aggregates with the given Aggregate UUID.
//
// Project must not be called before Run has been called. Otherwise the projection
// Job won't be executed by p.applyJob, because it hasn't been subscribed to the
// Schedule yet.
func (p *TimelineProjector) Project(ctx context.Context, id uuid.UUID) error {
	return p.schedule.Trigger(ctx, query.New(query.Aggregate(AggregateName, id)))
}

// applyJob applies a projection Job for Timelines.
func (p *TimelineProjector) applyJob(j project.Job) error {
	// First we extract the Order UUIDs from the Jobs Events. The Order UUIDs
	// are extracted from the Events that triggered the Job.
	orderIDs, err := j.AggregatesOf(j.Context(), AggregateName)
	if err != nil {
		return fmt.Errorf("extract Order IDs from Job: %w", err)
	}

	// For every Order UUID extracted from the Jobs Events, project the Timeline
	// for that Order:
	for _, id := range orderIDs {
		// First we try to fetch an existing Timeline for that Order.
		tl, err := p.repo.Fetch(j.Context(), id)
		if err != nil {
			if !errors.Is(err, ErrTimelineNotFound) {
				return fmt.Errorf("fetch Timeline %s: %w", id, err)
			}

			// If the Timeline does not exist yet, create it.
			tl = NewTimeline(id)
		}

		// Apply the Job on the Timeline. Note that a Job can be applied to as
		// many projections as you want. The Job will query the needed Events
		// on-the-fly for each projection.
		if err := j.Apply(j.Context(), tl); err != nil {
			return fmt.Errorf("apply Job: %w", err)
		}

		// Save the Timeline to the TimelineRepository.
		if err := p.repo.Save(j.Context(), tl); err != nil {
			return fmt.Errorf("save Timeline: %w", err)
		}
	}

	return nil
}
