package project

import "github.com/modernice/goes/event"

type Projection struct {
	latest event.Event
}

func NewProjection() *Projection {
	return &Projection{}
}

func (p *Projection) PostApplyEvent(evt event.Event) {
	p.latest = evt
}

func (p *Projection) LatestEvent() event.Event {
	return p.latest
}

func (p *Projection) ApplyEvent(event.Event) {}
