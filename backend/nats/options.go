package nats

import (
	"time"

	"github.com/modernice/goes"
	"github.com/nats-io/nats.go"
)

// Use returns the option to specify the Driver to use to communicate with NATS.
// By default, the Core Driver is used.
//
//	bus := NewEventBus(enc, Use(JetStream()))
func Use[ID goes.ID](d Driver[ID]) EventBusOption {
	return func(opts *eventBusOptions) {
		opts.driver = d
	}
}

// URL returns an option that sets the connection URL to the NATS server. If no
// URL is specified, the environment variable `NATS_URL` will be used as the
// connection URL. If that is also not set, the default NATS URL
// (nats.DefaultURL) is used instead.
func URL(url string) EventBusOption {
	return func(opts *eventBusOptions) {
		opts.url = url
	}
}

// Conn returns an option that provides the underlying *nats.Conn to the
// event bus. When providing a connection, the event bus does not try to connect
// to NATS but uses the provided connection instead.
func Conn(conn *nats.Conn) EventBusOption {
	return func(opts *eventBusOptions) {
		opts.conn = conn
	}
}

// EatErrors returns an option that discards any asynchronous errors of
// subscriptions. When subscribing to an event, you can safely ignore the
// returned error channel:
//
//	var bus *EventBus
//	events, _, err := bus.Subscribe(context.TODO(), ...)
func EatErrors() EventBusOption {
	return func(opts *eventBusOptions) {
		opts.eatErrors = true
	}
}

// QueueGroupByFunc returns an option that specifies the NATS queue group for
// new subscriptions. When subscribing to an event, fn(eventName) is called to
// determine the queue group name for that subscription. If the returned queue
// group is an empty string, the queue group feature will not be used for the
// subscription.
//
// Use Case
//
// Queue groups can be used to load-balance between multiple subscribers of the
// same event. When multiple subscribers are subscribed to an event and use the
// same queue group, only one of the subscribers will receive the event.
// To load-balance between instances of a replicated (micro-)service, use a
// shared name (service name) that is the same between the replicated services
// and use that id as the queue group:
//
//	serviceName := "foo-service"
//	bus := NewEventBus(enc,
//		QueueGroupByFunc(func(eventName) string {
//			return serviceName
//		}),
//	)
//
// The example above can also be written as:
//
//	serviceName := "foo-service"
//	bus := NewEventBus(enc, WithLoadBalancer(serviceName))
//
// Queue groups are disabled by default.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/core-nats/queue
func QueueGroupByFunc(fn func(eventName string) string) EventBusOption {
	return func(opts *eventBusOptions) {
		opts.queueFunc = fn
	}
}

// QueueGroupByFunc returns an option that specifies the NATS queue group for
// new subscriptions. When subscribing to an event, the event name is used as
// the name of the queue group.
//
// Use this option if you want events to be received only by a single subscriber.
//
// Can also be set with the `NATS_QUEUE_GROUP_BY_EVENT=1` environment variable.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/core-nats/queue
func QueueGroupByEvent() EventBusOption {
	return QueueGroupByFunc(func(eventName string) string {
		return eventName
	})
}

// QueueGroupBy returns an option that specifies the NATS queue group for new
// subscriptions.
//
// Can also be set with the `NATS_QUEUE_GROUP=foo` environment variable.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/core-nats/queue
func QueueGroup(queue string) EventBusOption {
	return QueueGroupByFunc(func(string) string {
		return queue
	})
}

// WithLoadBalancer returns a QueueGroupByFunc option that load-balances events
// between instances of a replicted (micro-)service. The provided serviceName is
// used as the queue group name. Any "." in serviceName are replaced with "_".
//
// Can also be set with the `NATS_LOAD_BALANCER=foo` environment variable.
//
// Caution
//
// Providing a load-balanced event bus as the underlying bus to a command bus
// should be avoided and providing it to a projection schedule should be done
// with thought and caution. Create another instance of an event bus without
// this option and pass that to cmdbus.New() when creating the command bus. When
// you create a projection schedule, you have to think about what makes sense in
// the context of your projection, because a load-balanced event bus will cause
// only a single instance of a replicated service to trigger a projection. Also,
// each event may be received by a different instance which can make the
// projection jobs fragmented and less efficient. A common example is some kind
// of lookup table that is projected from events and that your instances keep
// "live" in-memory. Each instance needs the lookup table to work, so it
// wouldn't make sense to load-balance the projection. In most cases where
// projections are not kept in memory but instead fetched from a database,
// updated and then saved back to the database, a load-balanced projection
// schedule is exactly what you want, but then again, context matters.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/core-nats/queue
func WithLoadBalancer(serviceName string) EventBusOption {
	return QueueGroupByFunc(func(string) string {
		return replaceDots(serviceName)
	})
}

// SubjectFunc returns an option that specifies how the NATS subjects for event
// names are generated.
//
// By default, subjects are the event names with "." replaced with "_".
func SubjectFunc(fn func(eventName string) string) EventBusOption {
	return func(opts *eventBusOptions) {
		opts.subjectFunc = func(eventName string) string {
			return replaceDots(fn(eventName))
		}
	}
}

// SubjectFunc returns an option that specifies how the NATS subjects for event
// names are generated.
//
// Can also be set with the `NATS_SUBJECT_PREFIX` environment variable.
func SubjectPrefix(prefix string) EventBusOption {
	return SubjectFunc(func(eventName string) string {
		return replaceDots(prefix + eventName)
	})
}

// DurableFunc returns an option that specifies the durable name for new
// subscriptions when using the JetStream Driver. When subscribing to an event,
// the provided function is called with the event name and queue group (see
// QueueGroupByXXX and WithLoadBalancer options) and the returned string is used
// as the durable name for the subscription. If the durable name is an empty
// string, the subscription is not made durable.
//
// Can also be set with the `NATS_DURABLE_NAME` environment variable. The
// following example generates the durable names by concatenating the subject
// together with the queue group using an underscore (the environment variable
// is executed using text/template, so you have access to the subject and queue
// group):
//
//	`NATS_DURABLE_NAME={{ .Subject }}_{{ .Queue }}`
//
// This option is valid only for the JetStream Driver.
//
// Use Case
//
// The following example uses durable subscriptions while load-balancing between
// instances of a replicated (micro-)service:
//
//	serviceName := "foo-service"
//	bus := NewEventBus(enc,
//		WithLoadBalancer(serviceName),
//		Durable(serviceName),
//	)
//
// Read more about durable subscriptions:
// https://docs.nats.io/nats-concepts/jetstream/consumers#durable-name
func DurableFunc(fn func(subject, queue string) string) EventBusOption {
	return func(opts *eventBusOptions) {
		opts.durableFunc = fn
	}
}

// Durable returns an option that specifies the durable name for new
// subscriptions when using the JetStream Driver.
//
// Use the DurableFunc option if you need to know the subject or queue group to
// build the durable name.
//
// This option is valid only for the JetStream Driver.
func Durable(name string) EventBusOption {
	return DurableFunc(func(_, _ string) string {
		return name
	})
}

// StreamNameFunc returns an option that specifies the stream name for new
// subscriptions when using the JetStream Driver. When subscribing to an event,
// the provided fn is called with the generated subject and queue group to
// determine the JetStream stream name for the subscription.
//
// If the StreamNameFunc option is not used, the provided DurableXXX option is
// used to generate the stream name instead. If the generated durable name is
// empty, the subscription falls back to using the default stream name function,
// which is the default durable name function.
//
// This option is valid only for the JetStream Driver.
//
// Read more about streams: https://docs.nats.io/nats-concepts/jetstream/streams
func StreamNameFunc(fn func(subject, queue string) string) EventBusOption {
	return func(opts *eventBusOptions) {
		opts.streamNameFunc = fn
	}
}

// SubOpts returns an option that adds custom nats.SubOpts when creating a
// JetStream subscription.
//
// This option is valid only for the JetStream Driver.
func SubOpts(opts ...nats.SubOpt) EventBusOption {
	return func(bopts *eventBusOptions) {
		bopts.subOpts = append(bopts.subOpts, opts...)
	}
}

// PullTimeout returns an Option that limits the duration the EventBus tries
// to send Events into the channel returned by bus.Subscribe. When d is exceeded
// the Event will be dropped. The default is a duration of 0 and means no timeout.
//
// Can also be set with the "NATS_RECEIVE_TIMEOUT" environment variable in a
// format understood by time.ParseDuration. If the environment value is not
// parseable by time.ParseDuration, no timeout will be used.

// PullTimeout returns an option that limits the Duration an EventBus tries to
// push an event into a subscribed event channel. When the pull timeout is
// exceeded, the event gets dropped and a warning is logged.
//
// Default is no timeout.
func PullTimeout(d time.Duration) EventBusOption {
	return func(opts *eventBusOptions) {
		opts.pullTimeout = d
	}
}
