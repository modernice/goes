package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/tuple"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query"
	"github.com/modernice/goes/projection"
	"github.com/modernice/goes/projection/schedule"
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
	// Embedding a *projection.Progressor (or implementing projection.progressor) is
	// needed if a projection needs to be resumed from where its last Event has
	// been applied. Otherwise Events that have already been applied would be
	// applied again with the next projection Job.
	//
	// See TimelineProjector.applyJob for more information.
	*projection.Progressor

	// A Guard guards the projection from Events. Guard receives Events before
	// they are applied to the Timeline and determines if the Event should be
	// applied.
	//
	// View NewTimeline to see how to setup a Guard.
	projection.Guard `bson:"-"`

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
	schedule projection.Schedule
}

// NewTimeline returns the Timeline for the Order with the given UUID.
func NewTimeline(id uuid.UUID) *Timeline {
	return &Timeline{
		Progressor: &projection.Progressor{},
		Guard: projection.QueryGuard(query.New(
			query.Aggregate(AggregateName, id),
		)),
		ID: id,
	}
}

// Duration returns the total lifetime of an Order.
func (tl *Timeline) Duration() (time.Duration, bool) {
	if tl.CanceledAt.IsZero() {
		return 0, false
	}
	return tl.CanceledAt.Sub(tl.PlacedAt), true
}

// // GuardProjection guards the projection from Events. Guard receives Events
// // before they are applied to the Timeline and determines if the Event should be
// // applied.
// //
// // Here we are validating that we only apply Event of the "Order" Aggregate and
// // from those Events only those that belong to the Order with the UUID equal to
// // the UUID from the Timeline.
// //
// // Implementing GuardProjection is optional.
// func (tl *Timeline) GuardProjection(evt event.Event) bool {
// 	switch evt.AggregateName() {
// 	case AggregateName: // "order"
// 		return evt.AggregateID() == tl.ID
// 	default:
// 		return false
// 	}
// }

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
	s := schedule.Continuously(bus, store, Events[:], schedule.Debounce(100*time.Millisecond))

	return &TimelineProjector{
		repo:     repo,
		schedule: s,
	}
}

// Run starts the TimelineProjector in the background and returns a channel of
// projection errors. Callers must receive from the error channel to prevent
// goroutine blocking.
func (p *TimelineProjector) Run(ctx context.Context) (<-chan error, error) {
	return p.schedule.Subscribe(ctx, p.applyJob)
}

// All projects the Timeline for every Order. This works by triggering the
// projection Schedule that was subscribed to by p.Run. When All is called,
// p.applyJob received a projection Job for every Event that is one of the
// configured Events in the Schedule (which in this case are all Order Events).
// This means that EVERY Order Event in the Event Store is fetched when applying
// the Job to a Timeline.
//
// All must not be called before Run has returned. Otherwise the projection Job
// won't be executed by p.applyJob, because it hasn't been subscribed to the
// Schedule yet.
func (p *TimelineProjector) All(ctx context.Context) error {
	return p.schedule.Trigger(ctx)
}

// Project projects the Timeline for the Order with the given UUID. This works
// by triggering the projection Schedule with an additional Event Query that
// only allows Events of Order Aggregates with the given Aggregate UUID. The
// Queries passed to Trigger are merged with the default Query from the Job and
// that merged Query is used to query Events for a specific Timeline t when the
// triggered projection Job is applied onto t.
//
// Project must not be called before Run has been called. Otherwise the projection
// Job won't be executed by p.applyJob, because it hasn't been subscribed to the
// Schedule yet.
func (p *TimelineProjector) Project(ctx context.Context, id uuid.UUID) error {
	return p.schedule.Trigger(ctx, projection.Filter(query.New(
		// If an Event is an Order Event, only allow it if its UUID is equal to
		// the provided UUID:
		query.Aggregate(AggregateName, id),
	)))
}

// applyJob applies a projection Job for Timelines.
func (p *TimelineProjector) applyJob(j projection.Job) error {
	// First we extract the Order UUIDs from the Jobs Events.
	// Note that a Job can contain Events belonging to multiple or even
	// different Aggregates.
	str, errs, err := j.Aggregates(j, AggregateName)
	if err != nil {
		return fmt.Errorf("extract aggregates from job: %w", err)
	}

	tuples, err := aggregate.DrainTuples(j, str, errs)
	if err != nil {
		return fmt.Errorf("drain aggregates: %w", err)
	}

	orderIDs := tuple.IDs(tuples...)

	// For every Order UUID extracted from the Jobs Events, project the Timeline
	// for that Order:
	for _, id := range orderIDs {
		// First we try to fetch an existing Timeline for that Order.
		tl, err := p.repo.Fetch(j, id)
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
		//
		// Apply uses the projection progress provided by a projection to change
		// the Query that is used to query the Events for the passed projection.
		// Assuming that Timeline tl has already has Events applied to it,
		// j.Apply will query only Events that happened after the last applied
		// Event. This works because Timeline embeds *projection.Progressor.
		if err := j.Apply(j, tl); err != nil {
			return fmt.Errorf("apply Job: %w", err)
		}

		// Save the Timeline to the TimelineRepository.
		if err := p.repo.Save(j, tl); err != nil {
			return fmt.Errorf("save Timeline: %w", err)
		}
	}

	return nil
}
