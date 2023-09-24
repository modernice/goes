package mongo

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/modernice/goes/event"
)

var _ Transaction = (*transaction)(nil)

type transactionKey struct{}

type transaction struct {
	mux            sync.RWMutex
	session        mongo.Session
	store          *EventStore
	insertedEvents []event.Event
}

func newTransaction(session mongo.Session, store *EventStore) *transaction {
	tx := &transaction{session: session}

	tx.store = &EventStore{
		isTransactionStore: true,
		root:               store,
		tx:                 tx,
	}

	return tx
}

// Session retrieves the mongo.Session associated with the current transaction.
// This is the session used for executing MongoDB operations within the
// transaction.
func (tx *transaction) Session() mongo.Session {
	return tx.session
}

// EventStore returns the associated event store of the transaction. The
// returned event store is a transactional store, which means that changes to
// the store are only visible within the scope of the transaction until it's
// committed.
func (tx *transaction) EventStore() *EventStore {
	return tx.store
}

// InsertedEvents returns a slice of all the events that have been inserted in
// the current transaction. The events are returned in the order they were
// inserted. This method is safe for concurrent use.
func (tx *transaction) InsertedEvents() []event.Event {
	tx.mux.RLock()
	defer tx.mux.RUnlock()

	copiedEvents := make([]event.Event, len(tx.insertedEvents))
	copy(copiedEvents, tx.insertedEvents)

	return copiedEvents
}

func (tx *transaction) appendEvents(events []event.Event) {
	tx.mux.Lock()
	defer tx.mux.Unlock()
	tx.insertedEvents = append(tx.insertedEvents, events...)
}

type transactionContext struct {
	context.Context
	Transaction
}

func newTransactionContext(ctx mongo.SessionContext, tx Transaction) *transactionContext {
	return &transactionContext{
		Context:     context.WithValue(ctx, transactionKey{}, tx),
		Transaction: tx,
	}
}

// TransactionFromContext retrieves the Transaction from the provided context.
// If no Transaction exists in the context, it returns nil.
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
