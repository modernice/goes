package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// Use returns the option to specify the Driver to use to communicate with NATS.
// By default, the "core" driver is used.
//
//	bus := NewEventBus(enc, Use(JetStream()))
func Use(d Driver) EventBusOption {
	return func(bus *EventBus) {
		bus.driver = d
	}
}

// URL returns an option that sets the connection URL to the NATS server. If no
// URL is specified, the environment variable `NATS_URL` will be used as the
// connection URL. If that is also not set, the default NATS URL
// (nats.DefaultURL) is used instead.
func URL(url string) EventBusOption {
	return func(bus *EventBus) {
		bus.url = url
	}
}

// Conn returns an option that provides the underlying *nats.Conn to the
// event bus. When providing a connection, the event bus does not try to connect
// to NATS but uses the provided connection instead.
func Conn(conn *nats.Conn) EventBusOption {
	return func(bus *EventBus) {
		bus.conn = conn
	}
}

// EatErrors returns an option that discards any asynchronous errors of
// subscriptions. When subscribing to an event, you can safely ignore the
// returned error channel:
//
//	var bus *EventBus
//	events, _, err := bus.Subscribe(context.TODO(), ...)
func EatErrors() EventBusOption {
	return func(bus *EventBus) {
		bus.eatErrors = true
	}
}

// QueueGroup returns an option that specifies the NATS queue group for
// new subscriptions. When subscribing to an event, fn(eventName) is called to
// determine the queue group name for that subscription. If the returned queue
// group is an empty string, the queue group feature will not be used for the
// subscription.
//
// This option has no effect if used with the "jetstream" driver in "pull" mode
// (default).
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
//		QueueGroup(func(eventName) string {
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
func QueueGroup(fn func(eventName string) string) EventBusOption {
	return func(bus *EventBus) {
		bus.queueFunc = fn
	}
}

// LoadBalancer returns a QueueGroup option that enables load-blancing
// between event buses that share the same serviceName. The option applies the
// QueueGroup option so that the queue group for the subscription to an event is
// built in the following format:
//	fmt.Sprintf("%s:%s", <serviceName>, <eventName>)
//
// Caution
//
// Providing a load-balanced event bus as the underlying bus to a command bus
// should be avoided, and providing it to a projection schedule should be done
// with caution.
//
// To create a command bus, create another instance of the event bus with load-
// balancing disabled, and pass that bus to cmdbus.New().
//
// When you create a projection schedule, you have to think about what makes
// sense in the context of your projection, because a load-balanced event bus
// will cause only a single instance of a replicated service to trigger a
// projection. Also, each event may be received by a different instance which
// can make the projection jobs fragmented and less efficient. A common example
// is some kind of lookup table that is projected from events and that your
// instances keep "live" in-memory. Each instance needs the lookup table to
// work, so it wouldn't make sense to load-balance the projection. In most cases
// where projections are not kept in memory but instead fetched from a database,
// updated and then saved back to the database, a load-balanced projection
// schedule is exactly what you want, but then again, context matters.
//
// Read more about queue groups: https://docs.nats.io/nats-concepts/core-nats/queue
func LoadBalancer(serviceName string) EventBusOption {
	return QueueGroup(func(eventName string) string {
		return fmt.Sprintf("%s:%s", serviceName, eventName)
	})
}

// SubjectFunc returns an option that specifies how the NATS subjects for event
// names are generated. Any "." in the subject are replaced by "_".
//
// By default, a subject is the event name with "." replaced by "_".
func SubjectFunc(fn func(eventName string) string) EventBusOption {
	return func(bus *EventBus) {
		bus.subjectFunc = func(eventName string) string {
			return replaceDots(fn(eventName))
		}
	}
}

// SubjectFunc returns an option that specifies how the NATS subjects for event
// names are generated.
func SubjectPrefix(prefix string) EventBusOption {
	return SubjectFunc(func(eventName string) string {
		return prefix + eventName
	})
}

// PullTimeout returns an option that limits the Duration an event bus tries to
// push an event into a subscribed event channel. When the pull timeout is
// exceeded, the event gets dropped and a warning is logged. Default is the
// zero-Duration which means "no timeout".
func PullTimeout(d time.Duration) EventBusOption {
	return func(bus *EventBus) {
		bus.pullTimeout = d
	}
}

func defaultSubjectFunc(eventName string) string {
	return replaceDots(eventName)
}

func noQueue(string) (q string) { return }
