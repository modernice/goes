package tagging

import "github.com/modernice/goes/event"

// Tagging events
const (
	Tagged   = "goes.tagging.tagged"
	Untagged = "goes.tagging.untagged"
)

type TaggedEvent struct {
	Tags []string
}

type UntaggedEvent struct {
	Tags []string
}

func RegisterEvents(r event.Registry) {
	r.Register(Tagged, func() event.Data { return TaggedEvent{} })
	r.Register(Untagged, func() event.Data { return UntaggedEvent{} })
}
