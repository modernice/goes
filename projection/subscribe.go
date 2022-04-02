package projection

// SubscribeOption is an option for Schedule.Subscribe.
type SubscribeOption func(*Subscription)

// Subscripion is the configuration for a subscription to a projection schedule.
type Subscription struct {
	// If provided, the projection schedule triggers a projection job on startup.
	// The projection job's `Aggregates()` and `Aggregate()` helpers will use
	// this query to extract the aggregates from the event store. This allows to
	// optimize the query performance of initial projection runs, which often
	// times need to fetch the ids of all aggregates of a specific kind from the
	// event store in order to get all projections up-to-date.
	Startup *Trigger
}

// Startup returns a SubscribeOption that triggers an initial projection run
// when subscribing to a projection schedule.
func Startup(opts ...TriggerOption) SubscribeOption {
	return func(cfg *Subscription) {
		t := NewTrigger(opts...)
		cfg.Startup = &t
	}
}

// NewSubscription creates a Subscription using the provided options.
func NewSubscription(opts ...SubscribeOption) Subscription {
	var sub Subscription
	for _, opt := range opts {
		opt(&sub)
	}
	return sub
}
