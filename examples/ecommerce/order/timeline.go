package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/project"
	"github.com/modernice/goes/event"
)

var (
	ErrTimelineNotFound = errors.New("timeline not found")
)

type TimelineRepository interface {
	Save(context.Context, *Timeline) error
	Fetch(context.Context, uuid.UUID) (*Timeline, error)
}

type Timeline struct {
	project.Projection `bson:"-"`

	ID         uuid.UUID `bson:"id"`
	PlacedAt   time.Time `bson:"placedAt"`
	CanceledAt time.Time `bson:"canceledAt"`
	Steps      []Step    `bson:"steps"`
}

type Step struct {
	Start time.Time `bson:"start"`
	End   time.Time `bson:"end"`
	Desc  string    `bson:"desc"`
}

func NewTimeline(id uuid.UUID) *Timeline {
	return &Timeline{
		Projection: project.New(AggregateName, id),
		ID:         id,
	}
}

func ProjectTimeline(
	ctx context.Context,
	repo TimelineRepository,
	proj project.Projector,
	bus event.Bus,
) (<-chan error, error) {
	s := project.Continuously(bus, Events[:])
	str, subErrs, err := project.Subscribe(ctx, s, proj)
	if err != nil {
		return nil, fmt.Errorf("subscribe: %w", err)
	}

	out := make(chan error)

	go func() {
		defer close(out)
		for {
			select {
			case ctx, ok := <-str:
				if !ok {
					return
				}
				tl := NewTimeline(ctx.AggregateID())
				if err := ctx.Project(ctx, tl); err != nil {
					out <- err
					break
				}
				if err := repo.Save(ctx, tl); err != nil {
					out <- fmt.Errorf("save Timeline: %w", err)
				}
			case err, ok := <-subErrs:
				if !ok {
					subErrs = nil
					break
				}
				out <- err
			}
		}
	}()

	return out, nil
}

func (tl *Timeline) Duration() (time.Duration, bool) {
	if tl.CanceledAt.IsZero() {
		return 0, false
	}
	return tl.CanceledAt.Sub(tl.PlacedAt), true
}

func (tl *Timeline) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case Placed:
		tl.placed(evt)
	case Canceled:
		tl.canceled(evt)
	}
	tl.finalizeSteps()
}

func (tl *Timeline) placed(evt event.Event) {
	data := evt.Data().(PlacedEvent)
	tl.PlacedAt = evt.Time()
	tl.Steps = append(tl.Steps, Step{
		Start: evt.Time(),
		Desc:  fmt.Sprintf("%s placed an Order for %d Items at %s.", data.Customer.Name, len(data.Items), evt.Time()),
	})
}

func (tl *Timeline) canceled(evt event.Event) {
	tl.CanceledAt = evt.Time()
	tl.Steps = append(tl.Steps, Step{
		Start: evt.Time(),
		Desc:  fmt.Sprintf("Order was canceled at %s.", evt.Time()),
	})
}

func (tl *Timeline) finalizeSteps() {
	for i, step := range tl.Steps {
		if i == 0 {
			continue
		}
		tl.Steps[i-1].End = step.Start
	}
}
