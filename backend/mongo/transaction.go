package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/modernice/goes/event"
)

type transaction struct {
	session mongo.SessionContext
	Events  []event.Event
	store   *EventStore
}

// NewTransaction creates a new transaction
func NewTransaction(sessionCtx mongo.SessionContext, store *EventStore) *transaction {
	return &transaction{
		session: sessionCtx,
		store:   store,
	}
}

var _ Transaction = (*transaction)(nil)

// Session returns the mongo session context
func (t *transaction) Session() mongo.SessionContext {
	return t.session
}

// EventStore returns the event store itself to allow inserting (or deleting)
// more events within the same transaction.
func (t *transaction) EventStore() *EventStore {
	return t.store
}

// InsertedEvents returns all events that were inserted during this transaction at a given point.
// They won't be inserted if the transaction is aborted.
// When calling EventStore().Insert(), the inserted events will be added to this list.
func (t *transaction) InsertedEvents() []event.Event {
	return t.Events
}

// ApplyEvents is a way for the transaction to be able to add newly inserted events to the transaction.
func (t *transaction) ApplyEvents(events []event.Event) {
	t.Events = append(t.Events, events...)
}

type transactionContext struct {
	context.Context
	Transaction
}

// Session implements the Transaction interface
func (t *transactionContext) Session() mongo.SessionContext {
	return t.Transaction.Session()
}

// EventStore implements the Transaction interface
func (t *transactionContext) EventStore() *EventStore {
	return t.Transaction.EventStore()
}

// InsertedEvents implements the Transaction interface
func (t *transactionContext) InsertedEvents() []event.Event {
	return t.Transaction.InsertedEvents()
}

type transactionKey struct{}

// NewTransactionContext creates a new transaction context with the given transaction inside.
func NewTransactionContext(ctx context.Context, tx Transaction) *transactionContext {
	return &transactionContext{
		Context:     context.WithValue(ctx, transactionKey{}, tx),
		Transaction: tx,
	}
}

// TransactionFromContext retrieves an existing transaction from context or returns nil otherwise.
func TransactionFromContext(ctx context.Context) Transaction {
	val := ctx.Value(transactionKey{})
	if val == nil {
		return nil
	}

	tx, ok := val.(Transaction)
	if !ok {
		return nil
	}

	return tx
}
