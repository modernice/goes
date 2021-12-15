package nats

import (
	"github.com/nats-io/nats.go"
)

// Use returns an Option that specifies which Driver to use to communicate with
// NATS. Defaults to Core().
func Use(d Driver) EventBusOption {
	return func(bus *EventBus) {
		bus.driver = d
	}
}

// QueueGroupByFunc returns an Option that sets the NATS queue group for
// subscriptions by calling fn with the name of the subscribed Event. This can
// be used to load-balance Events between subscribers.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/queue
func QueueGroupByFunc(fn func(eventName string) string) EventBusOption {
	return func(bus *EventBus) {
		bus.queueFunc = fn
	}
}

// QueueGroupByEvent returns an Option that sets the NATS queue group for
// subscriptions to the name of the handled Event. This can be used to
// load-balance Events between subscribers of the same Event name.
//
// Can also be set with the "NATS_QUEUE_GROUP_BY_EVENT" environment variable.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/queue
func QueueGroupByEvent() EventBusOption {
	return QueueGroupByFunc(func(eventName string) string {
		return eventName
	})
}

// SubjectFunc returns an Option that sets the NATS subject for subscriptions
// and outgoing Events by calling fn with the name of the handled Event.
func SubjectFunc(fn func(eventName string) string) EventBusOption {
	return func(bus *EventBus) {
		bus.subjectFunc = fn
	}
}

// SubjectPrefix returns an Option that sets the NATS subject for subscriptions
// and outgoing Events by prepending prefix to the name of the handled Event.
//
// Can also be set with the "NATS_SUBJECT_PREFIX" environment variable.
func SubjectPrefix(prefix string) EventBusOption {
	return SubjectFunc(func(eventName string) string {
		return prefix + eventName
	})
}

// DurableFunc returns an Option that sets fn as the function to build the
// DurableName for the NATS Streaming subscriptions. When fn return an empty
// string, the subscription will not be made durable.
//
// DurableFunc has no effect when using the NATS Core Driver because NATS Core
// doesn't support durable subscriptions.
//
// Can also be set with the "NATS_DURABLE_NAME" environment variable:
//	`NATS_DURABLE_NAME={{ .Subject }}_{{ .Queue }}`
//
// Read more about durable subscriptions:
// https://docs.nats.io/developing-with-nats-streaming/durables
func DurableFunc(fn func(subject, queueGroup string) string) EventBusOption {
	return func(bus *EventBus) {
		bus.durableFunc = fn
	}
}

// Durable returns an Option that makes the NATS subscriptions durable.
//
// If the queue group is not empty, the durable name is built by concatenating
// the subject and queue group with an underscore:
//	fmt.Sprintf("%s_%s", subject, queueGroup)
//
// If the queue group is an empty string, the durable name is set to the
// subject.
//
// Can also be set with the "NATS_DURABLE_NAME" environment variable:
//	`NATS_DURABLE_NAME={{ .Subject }}_{{ .Queue }}`
//
// Use DurableFunc instead to control how the durable name is built.
func Durable() EventBusOption {
	return DurableFunc(defaultDurableNameFunc)
}

func StreamNameFunc(fn func(subject, queue string) string) EventBusOption {
	return func(bus *EventBus) {
		bus.streamNameFunc = fn
	}
}

func SubOpts(opts ...nats.SubOpt) EventBusOption {
	return func(bus *EventBus) {
		bus.subOpts = append(bus.subOpts, opts...)
	}
}

// URL returns an Option that sets the connection URL to the NATS server. If no
// URL is specified, the environment variable "NATS_URL" will be used as the
// connection URL.
//
// Can also be set with the "NATS_URL" environment variable.
func URL(url string) EventBusOption {
	return func(bus *EventBus) {
		bus.url = url
	}
}

// Conn returns an Option that provides the underlying *nats.Conn for the
// EventBus. When the Conn Option is used, the Use Option has no effect.
func Conn(conn *nats.Conn) EventBusOption {
	return func(bus *EventBus) {
		bus.conn = conn
	}
}
