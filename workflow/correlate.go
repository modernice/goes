package workflow

import (
	"github.com/google/uuid"
	"github.com/modernice/goes/event"
)

// A Correlator finds the workflow instance that should handle a trigger
// event. It returns the id of the workflow and whether the event correlates
// to a workflow at all. A Correlator is just a function — write your own to
// correlate by whatever the event provides:
//
//	func byOrder(evt event.Of[PaymentReceivedData]) (uuid.UUID, bool) {
//		return evt.Data().OrderID, true
//	}
type Correlator[Data any] func(event.Of[Data]) (uuid.UUID, bool)

// ByAggregateID correlates trigger events to the workflow that has the same
// id as the aggregate that emitted the event — one workflow instance per
// aggregate. Events without an aggregate are ignored.
//
// ByAggregateID is passed uninstantiated; its type argument is inferred
// from the handler of the registration:
//
//	workflow.Starts(workflow.ByAggregateID, (*OrderWorkflow).onPlaced, OrderPlaced)
func ByAggregateID[Data any](evt event.Of[Data]) (uuid.UUID, bool) {
	id, _, _ := evt.Aggregate()
	return id, id != uuid.Nil
}

// ByKey returns a Correlator that derives the workflow id from a business
// key in the event payload, using deterministic (UUIDv5) derivation within
// the given namespace — one workflow instance per key. All events that
// yield the same key correlate to the same workflow, regardless of which
// aggregate emitted them. Events for which the key function returns an
// empty string are ignored.
//
//	var customers = uuid.MustParse("d3f0a1de-52a7-40e8-8a3a-79dbd8f2071e")
//
//	workflow.Starts(
//		workflow.ByKey(customers, func(d OrderPlacedData) string { return d.CustomerEmail }),
//		(*LoyaltyWorkflow).onOrder,
//		OrderPlaced,
//	)
func ByKey[Data any](namespace uuid.UUID, key func(Data) string) Correlator[Data] {
	return func(evt event.Of[Data]) (uuid.UUID, bool) {
		k := key(evt.Data())
		if k == "" {
			return uuid.Nil, false
		}
		return uuid.NewSHA1(namespace, []byte(k)), true
	}
}
