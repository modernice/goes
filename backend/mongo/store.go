package mongo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	stdtime "time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver"

	"github.com/modernice/goes/backend/mongo/indices"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/helper/pick"
)

var (
	// PreInsert represents a hook that is executed before inserting events into the
	// EventStore. This allows for additional operations to be performed within the
	// same transaction, such as validation or transformation of data. The hook
	// function should return an error if anything goes wrong, causing the
	// transaction to abort.
	PreInsert = TransactionHook("pre:insert")

	// PostInsert represents a hook that is executed after inserting events into the
	// EventStore. This allows for additional operations to be performed within the
	// same transaction, such as cleanup or logging operations. The hook function
	// should return an error if anything goes wrong, causing the transaction to
	// abort.
	PostInsert = TransactionHook("post:insert")
)

// EventStore is a type that provides an interface to store, retrieve, and
// manage events in a MongoDB database. It supports insertion and deletion of
// events, querying for specific events, and consistency checks through version
// validation. EventStore also allows for the use of hooks that can be executed
// before or after inserting events into the store. It provides transactional
// operations to ensure atomicity and consistency of the stored data. The
// EventStore can be configured with various options such as MongoDB connection
// details, collections for storing events and aggregate states, and the use of
// transactions.
type EventStore struct {
	enc               codec.Encoding
	url               string
	dbname            string
	entriesCol        string
	statesCol         string
	noIndex           bool
	transactions      bool
	validateVersions  bool
	additionalIndices []mongo.IndexModel
	preInsertHooks    []func(TransactionContext) error
	postInsertHooks   []func(TransactionContext) error

	client  *mongo.Client
	db      *mongo.Database
	entries *mongo.Collection
	states  *mongo.Collection

	isTransactionStore bool
	tx                 *transaction
	root               *EventStore

	onceConnect sync.Once
}

// EventStoreOption is a function that modifies an EventStore. These options are
// used to configure various aspects of the EventStore, such as the MongoDB
// instance connection details, the collections where events and aggregate
// states are stored, the use of transactions when inserting events, and more.
// It also supports the use of hooks that are executed before or after inserting
// events into the store.
type EventStoreOption func(*EventStore)

// VersionError represents an error that occurs when an event has an incorrect
// version. This usually happens when the event's version does not match the
// expected version based on the current state of its corresponding aggregate.
// This error is used within the EventStore to ensure consistency of events and
// aggregates. The fields in VersionError provide additional information about
// the error, including the name and ID of the aggregate, the current version of
// the aggregate, and the event that caused the error. Methods are provided to
// return a string representation of the error and to check if it is a
// consistency error.
type VersionError struct {
	AggregateName  string
	AggregateID    uuid.UUID
	CurrentVersion int
	Event          event.Event
	err            error
}

// Error returns a string representation of the VersionError. If an underlying
// error is present, it prepends "version error: " to the error message.
// Otherwise, it generates a message indicating that an event has an incorrect
// version compared to what was expected.
func (err VersionError) Error() string {
	if err.err != nil {
		return fmt.Sprintf("version error: %s", err.err)
	}

	return fmt.Sprintf(
		"event should have version %d, but has version %d",
		err.CurrentVersion+1,
		pick.AggregateVersion(err.Event),
	)
}

// IsConsistencyError checks if a VersionError is a consistency error. It always
// returns true as all VersionErrors are considered consistency errors. This
// method is useful for handling errors where consistency needs to be ensured,
// such as when the version of an event does not match the expected version
// based on the current state of its corresponding aggregate.
func (err VersionError) IsConsistencyError() bool {
	return true
}

// CommandError is a mongo.CommandError that satisfies aggregate.IsConsistencyError(err).
type CommandError mongo.CommandError

// CommandError CommandError returns a mongo.CommandError from the receiver. It
// is used for type conversion from CommandError to mongo.CommandError.
func (err CommandError) CommandError() mongo.CommandError {
	return mongo.CommandError(err)
}

//jotbot:ignore
func (err CommandError) Error() string {
	return mongo.CommandError(err).Error()
}

// IsConsistencyError reports whether a CommandError is a consistency error. A
// CommandError is considered a consistency error if it has an error label
// indicating that the operation failed due to a transient or unknown
// transaction error.
func (err CommandError) IsConsistencyError() bool {
	return true
}

type state struct {
	AggregateName string    `bson:"aggregateName"`
	AggregageID   uuid.UUID `bson:"aggregateId"`
	Version       int       `bson:"version"`
}

type entry struct {
	ID               uuid.UUID    `bson:"id"`
	Name             string       `bson:"name"`
	Time             stdtime.Time `bson:"time"`
	TimeNano         int64        `bson:"timeNano"`
	AggregateName    string       `bson:"aggregateName"`
	AggregateID      uuid.UUID    `bson:"aggregateId"`
	AggregateVersion int          `bson:"aggregateVersion"`
	Data             []byte       `bson:"data"`
}

// URL returns an Option that specifies the URL to the MongoDB instance. An
// empty URL means "use the default".
//
// Defaults to the environment variable "MONGO_URL".
func URL(url string) EventStoreOption {
	return func(s *EventStore) {
		s.url = url
	}
}

// Client returns an EventStoreOption that sets the provided mongo.Client to be
// used by the EventStore. This option is useful when you already have a
// mongo.Client and want to reuse it, instead of letting the EventStore create
// its own client.
func Client(c *mongo.Client) EventStoreOption {
	return func(s *EventStore) {
		s.client = c
	}
}

// Database returns an Option that sets the mongo database to use for the events.
func Database(name string) EventStoreOption {
	return func(s *EventStore) {
		s.dbname = name
	}
}

// Collection returns an Option that sets the mongo collection where the events
// are stored in.
func Collection(name string) EventStoreOption {
	return func(s *EventStore) {
		s.entriesCol = name
	}
}

// StateCollection returns an Option that specifies the name of the Collection
// where the current state of aggregates are stored in.
func StateCollection(name string) EventStoreOption {
	return func(s *EventStore) {
		s.statesCol = name
	}
}

// Transactions returns an Option that, if tx is true, configures a Store to use
// MongoDB Transactions when inserting events.
//
// Transactions can only be used in replica sets or sharded clusters:
// https://docs.mongodb.com/manual/core/transactions/
func Transactions(tx bool) EventStoreOption {
	return func(s *EventStore) {
		if !tx && s.transactions && (len(s.preInsertHooks) > 0 || len(s.postInsertHooks) > 0) {
			panic(fmt.Errorf("transactions must be enabled for transaction hooks"))
		}
		s.transactions = tx
	}
}

// ValidateVersions returns an Option that enables validation of event versions
// before inserting them into the Store.
//
// Defaults to true.
func ValidateVersions(v bool) EventStoreOption {
	return func(s *EventStore) {
		s.validateVersions = v
	}
}

// NoIndex returns an option to completely disable index creation when
// connecting to the event bus.
func NoIndex(ni bool) EventStoreOption {
	return func(es *EventStore) {
		es.noIndex = ni
	}
}

// WithIndices returns an EventStoreOption that creates additional indices for
// the event collection. Can be used to create builtin edge-case indices:
//
//	WithIndices(indices.EventStore.NameAndVersion)
func WithIndices(models ...mongo.IndexModel) EventStoreOption {
	return func(s *EventStore) {
		s.additionalIndices = append(s.additionalIndices, models...)
	}
}

// TransactionHook represents a hook that can be executed before or after
// inserting events into the EventStore. The hook function should return an
// error if anything goes wrong, causing the transaction to abort.
type TransactionHook string

// Transaction represents a set of operations that are executed within the same
// session in an EventStore. It encapsulates the session of the MongoDB driver
// and provides a reference to the associated EventStore. It also keeps track of
// all events that have been inserted into the EventStore within its session.
// Transactional operations ensure atomicity and consistency of data in the
// EventStore, making sure that either all operations are successfully
// completed, or none are in case of an error.
type Transaction interface {
	// Session retrieves the active MongoDB session associated with the current
	// transaction. This function allows for direct interaction with the MongoDB
	// session for operations not directly supported by the Transaction interface.
	// It returns a [mongo.Session] instance that represents the current session.
	// The Session method is only available within a Transaction and should not be
	// used outside of it to avoid unexpected behaviour or errors.
	Session() mongo.Session

	// EventStore is a method of the Transaction interface. It returns an instance
	// of the EventStore type that is associated with the transaction. This can be
	// used to perform operations on the event store within the context of the
	// transaction, ensuring consistency and atomicity of operations.
	EventStore() *EventStore

	// InsertedEvents retrieves all events that have been inserted into the store
	// within the current transaction. The function is part of the [Transaction]
	// interface and returns a slice of [event.Event]. This method is useful for
	// tracking changes made during a transaction, allowing for operations such as
	// rollbacks or additional processing based on the inserted events.
	InsertedEvents() []event.Event
}

// TransactionContext is an interface that embeds the standard context.Context
// and Transaction interfaces. It provides a way to pass transaction-specific
// data, such as the session information and inserted events, along with the
// usual context data. This is particularly useful when using hooks in the
// EventStore, allowing hook functions to access additional information about
// the ongoing transaction.
type TransactionContext interface {
	context.Context
	Transaction
}

// WithTransactionHook configures a transaction hook for the EventStore. The
// hook function will be called either before or after inserting events into the
// EventStore, depending on the specified TransactionHook. If the
// TransactionHook is "pre:insert", the hook function will be called before
// insertion. If it's "post:insert", the function will be called after
// insertion. The hook function should return an error if anything goes wrong,
// causing the transaction to abort.
func WithTransactionHook(hook TransactionHook, fn func(TransactionContext) error) EventStoreOption {
	return func(s *EventStore) {
		s.transactions = true
		switch hook {
		case PreInsert:
			s.preInsertHooks = append(s.preInsertHooks, fn)
		case PostInsert:
			s.postInsertHooks = append(s.postInsertHooks, fn)
		}
	}
}

// NewEventStore creates a new instance of an EventStore. This function accepts
// a codec.Encoding and any number of EventStoreOption functions. The EventStore
// instance created by this function will use the provided codec.Encoding for
// event serialization and deserialization. The EventStoreOptions are used to
// configure the behavior and characteristics of the EventStore, such as the
// underlying MongoDB client to use, the database and collections to store
// events in, and whether or not to validate event versions. If no database or
// collection names are provided via EventStoreOptions, default names will be
// used. By default, version validation is enabled.
func NewEventStore(enc codec.Encoding, opts ...EventStoreOption) *EventStore {
	s := EventStore{
		enc:              enc,
		validateVersions: true,
	}
	for _, opt := range opts {
		opt(&s)
	}
	if strings.TrimSpace(s.dbname) == "" {
		s.dbname = "event"
	}
	if strings.TrimSpace(s.entriesCol) == "" {
		s.entriesCol = "events"
	}
	if strings.TrimSpace(s.statesCol) == "" {
		s.statesCol = "states"
	}
	return &s
}

// Client returns the underlying mongo.Client. If no mongo.Client is provided
// with the Client option, Client returns nil until the connection to MongoDB
// has been established by either explicitly calling s.Connect or implicitly by
// calling s.Insert, s.Find, s.Delete or s.Query. Otherwise Client returns the
// provided mongo.Client.
func (s *EventStore) Client() *mongo.Client {
	return s.client
}

// Database returns the underlying mongo.Database. Database returns nil until
// the connection to MongoDB has been established by either explicitly calling
// s.Connect or implicitly by calling s.Insert, s.Find, s.Delete or s.Query.
func (s *EventStore) Database() *mongo.Database {
	return s.db
}

// Collection returns the underlying *mongo.Collection where the events are
// stored in. Collection returns nil until the connection to MongoDB has been
// established by either explicitly calling s.Connect or implicitly by calling
// s.Insert, s.Find, s.Delete or s.Query.
func (s *EventStore) Collection() *mongo.Collection {
	return s.entries
}

// StateCollection returns the underlying *mongo.Collection where aggregate
// states are stored in. StateCollection returns nil until the connection to
// MongoDB has been established by either explicitly calling s.Connect or
// implicitly by calling s.Insert, s.Find, s.Delete or s.Query.
func (s *EventStore) StateCollection() *mongo.Collection {
	return s.states
}

// Insert saves the given events into the database.
func (s *EventStore) Insert(ctx context.Context, events ...event.Event) (out error) {
	defer func() {
		if out == nil {
			return
		}

		var cmdError mongo.CommandError
		if errors.As(out, &cmdError) && (cmdError.HasErrorLabel(driver.TransientTransactionError) ||
			cmdError.HasErrorLabel(driver.UnknownTransactionCommitResult)) {
			out = CommandError(cmdError)
		}
	}()

	if s.isTransactionStore {
		return s.txInsert(ctx, events)
	}

	if err := s.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	tx, err := s.createTransaction(ctx)
	if err != nil {
		return err
	}
	defer tx.Session().EndSession(ctx)

	sessionCtx := mongo.NewSessionContext(ctx, tx.Session())

	if s.transactions {
		if err := sessionCtx.StartTransaction(); err != nil {
			return fmt.Errorf("start transaction: %w", err)
		}
	}

	txCtx := newTransactionContext(sessionCtx, tx)
	for _, hook := range s.preInsertHooks {
		if err := hook(txCtx); err != nil {
			return s.abortTransaction(sessionCtx, fmt.Errorf("pre-insert hook: %w", err))
		}
	}

	if err := s.insertInSession(sessionCtx, events); err != nil {
		return err
	}

	tx.appendEvents(events)

	for _, hook := range s.postInsertHooks {
		if err := hook(txCtx); err != nil {
			return s.abortTransaction(sessionCtx, fmt.Errorf("post-insert hook: %w", err))
		}
	}

	if s.transactions {
		if err := sessionCtx.CommitTransaction(sessionCtx); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
	}

	return nil
}

func (s *EventStore) txInsert(ctx context.Context, events []event.Event) error {
	if err := s.root.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	sessionCtx := mongo.NewSessionContext(ctx, s.tx.Session())

	if err := s.root.insertInSession(sessionCtx, events); err != nil {
		return err
	}

	s.tx.appendEvents(events)

	return nil
}

func (s *EventStore) createTransaction(ctx context.Context) (tx *transaction, err error) {
	session, err := s.client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}
	return newTransaction(session, s), nil
}

func (s *EventStore) abortTransaction(ctx mongo.SessionContext, err error) error {
	if s.isTransactionStore {
		return s.root.abortTransaction(ctx, err)
	}

	if !s.transactions {
		return err
	}

	if abortError := ctx.AbortTransaction(ctx); abortError != nil {
		return fmt.Errorf("abort transaction with error %q: %w", err, abortError)
	}

	return err
}

func (s *EventStore) insertInSession(ctx mongo.SessionContext, events []event.Event) (out error) {
	st, err := s.validateEventVersions(ctx, events)
	if err != nil {
		return s.abortTransaction(ctx, fmt.Errorf("validate versions: %w", err))
	}

	if err := s.insert(ctx, events); err != nil {
		return s.abortTransaction(ctx, err)
	}

	if err := s.updateState(ctx, st, events); err != nil {
		return s.abortTransaction(ctx, fmt.Errorf("update aggregate state: %w", err))
	}

	return nil
}

func (s *EventStore) validateEventVersions(ctx mongo.SessionContext, events []event.Event) (state, error) {
	if len(events) == 0 {
		return state{}, nil
	}

	aggregateID, aggregateName, aggregateVersion := events[0].Aggregate()

	if aggregateName == "" || aggregateID == uuid.Nil {
		return state{}, nil
	}

	res := s.states.FindOne(ctx, bson.D{
		{Key: "aggregateName", Value: aggregateName},
		{Key: "aggregateId", Value: aggregateID},
	})

	var st state
	if err := res.Decode(&st); err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return state{}, fmt.Errorf("decode state: %w", err)
		}
		st = state{
			AggregateName: aggregateName,
			AggregageID:   aggregateID,
		}
	}

	if !s.validateVersions {
		return st, nil
	}

	if st.Version >= aggregateVersion {
		return st, VersionError{
			AggregateName:  aggregateName,
			AggregateID:    aggregateID,
			CurrentVersion: st.Version,
			Event:          events[0],
		}
	}

	return st, nil
}

func (s *EventStore) updateState(ctx mongo.SessionContext, st state, events []event.Event) error {
	if len(events) == 0 || st.AggregateName == "" || st.AggregageID == uuid.Nil {
		return nil
	}
	st.Version = pick.AggregateVersion(events[len(events)-1])
	if _, err := s.states.ReplaceOne(
		ctx,
		bson.D{
			{Key: "aggregateName", Value: st.AggregateName},
			{Key: "aggregateId", Value: st.AggregageID},
		},
		st,
		options.Replace().SetUpsert(true),
	); err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	return nil
}

func (s *EventStore) insert(ctx context.Context, events []event.Event) error {
	if len(events) == 0 {
		return nil
	}

	docs := make([]any, len(events))
	for i, evt := range events {
		b, err := s.enc.Marshal(evt.Data())
		if err != nil {
			return fmt.Errorf("encode %q event data: %w", evt.Name(), err)
		}

		id, name, v := evt.Aggregate()
		docs[i] = entry{
			ID:               evt.ID(),
			Name:             evt.Name(),
			Time:             evt.Time(),
			TimeNano:         int64(evt.Time().UnixNano()),
			AggregateName:    name,
			AggregateID:      id,
			AggregateVersion: v,
			Data:             b,
		}
	}
	if _, err := s.entries.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	return nil
}

// Find returns the event with the specified UUID from the database if it exists.
func (s *EventStore) Find(ctx context.Context, id uuid.UUID) (event.Event, error) {
	if s.isTransactionStore {
		return s.root.Find(ctx, id)
	}

	if err := s.connectOnce(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	res := s.entries.FindOne(ctx, bson.M{"id": id})

	var e entry
	if err := res.Decode(&e); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}

	return e.event(s.enc)
}

// Delete deletes the given event from the database.
func (s *EventStore) Delete(ctx context.Context, events ...event.Event) error {
	if s.root != nil {
		return s.txDelete(ctx, events)
	}

	if len(events) == 0 {
		return nil
	}

	if err := s.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	tx, err := s.createTransaction(ctx)
	if err != nil {
		return err
	}
	defer tx.Session().EndSession(ctx)

	sessionCtx := mongo.NewSessionContext(ctx, tx.Session())

	if s.transactions {
		if err := sessionCtx.StartTransaction(); err != nil {
			return fmt.Errorf("start transaction: %w", err)
		}
	}

	commit := func() error {
		if s.transactions {
			if err := sessionCtx.CommitTransaction(ctx); err != nil {
				return fmt.Errorf("commit transaction: %w", err)
			}
		}
		return nil
	}

	ids := make([]uuid.UUID, len(events))
	for i, evt := range events {
		ids[i] = evt.ID()
	}

	if err := s.deleteInSession(sessionCtx, ids); err != nil {
		return err
	}

	name, id, version, hasAggregateData := checkDeletion(events)
	if !hasAggregateData {
		return commit()
	}

	if _, err := s.states.DeleteOne(ctx, bson.D{
		{Key: "aggregateName", Value: name},
		{Key: "aggregateId", Value: id},
		{Key: "aggregateVersion", Value: version},
	}); err != nil {
		return s.abortTransaction(sessionCtx, fmt.Errorf("delete aggregate state: %w", err))
	}

	return commit()
}

func (s *EventStore) deleteInSession(ctx mongo.SessionContext, ids []uuid.UUID) error {
	if _, err := s.entries.DeleteMany(ctx, bson.D{
		{Key: "id", Value: bson.D{{Key: "$in", Value: ids}}},
	}); err != nil {
		return s.abortTransaction(ctx, err)
	}
	return nil
}

func (s *EventStore) txDelete(ctx context.Context, events []event.Event) error {
	if len(events) == 0 {
		return nil
	}

	if err := s.root.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	sessionCtx := mongo.NewSessionContext(ctx, s.tx.Session())

	ids := make([]uuid.UUID, len(events))
	for i, evt := range events {
		ids[i] = evt.ID()
	}

	if err := s.root.deleteInSession(sessionCtx, ids); err != nil {
		return err
	}

	name, id, version, hasAggregateData := checkDeletion(events)
	if !hasAggregateData {
		return nil
	}

	if _, err := s.root.states.DeleteOne(ctx, bson.D{
		{Key: "aggregateName", Value: name},
		{Key: "aggregateId", Value: id},
		{Key: "aggregateVersion", Value: version},
	}); err != nil {
		return s.root.abortTransaction(sessionCtx, fmt.Errorf("delete aggregate state: %w", err))
	}

	return nil
}

// Query queries the database for events filtered by Query q and returns an
// streams.New for those events.
func (s *EventStore) Query(ctx context.Context, q event.Query) (<-chan event.Event, <-chan error, error) {
	if s.isTransactionStore {
		return s.root.Query(ctx, q)
	}

	if err := s.connectOnce(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}
	opts := options.Find().SetAllowDiskUse(true)
	opts = applySortings(opts, q.Sortings()...)

	f := makeFilter(q)

	cur, err := s.entries.Find(ctx, f, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("mongo: %w", err)
	}

	events := make(chan event.Event)
	errs := make(chan error)

	go func() {
		defer close(events)
		defer close(errs)

	L:
		for cur.Next(ctx) {
			var e entry
			if err := cur.Decode(&e); err != nil {
				select {
				case <-ctx.Done():
					return
				case errs <- err:
				}
				continue
			}
			evt, err := e.event(s.enc)
			if err != nil {
				select {
				case <-ctx.Done():
					break L
				case errs <- err:
					continue
				}
			}
			select {
			case <-ctx.Done():
				return
			case events <- evt:
			}
		}

		if err = cur.Err(); err != nil {
			select {
			case <-ctx.Done():
			case errs <- fmt.Errorf("mongo cursor: %w", err):
			}
		}
	}()

	return events, errs, nil
}

// Connect establishes the connection to the underlying MongoDB and returns the
// mongo.Client. Connect doesn't need to be called manually as it's called
// automatically on the first call to s.Insert, s.Find, s.Delete or s.Query. Use
// Connect if you want to explicitly control when to connect to MongoDB.
func (s *EventStore) Connect(ctx context.Context, opts ...*options.ClientOptions) (*mongo.Client, error) {
	if s.isTransactionStore {
		return s.root.Connect(ctx, opts...)
	}

	if err := s.connectOnce(ctx, opts...); err != nil {
		return nil, err
	}

	return s.client, nil
}

func (s *EventStore) connectOnce(ctx context.Context, opts ...*options.ClientOptions) error {
	var err error
	s.onceConnect.Do(func() {
		if err = s.connect(ctx, opts...); err != nil {
			return
		}

		if s.noIndex {
			return
		}

		if err = s.ensureIndexes(ctx); err != nil {
			err = fmt.Errorf("ensure indexes: %w", err)
			return
		}
	})
	return err
}

func (s *EventStore) connect(ctx context.Context, opts ...*options.ClientOptions) error {
	if s.client == nil {
		uri := s.url
		if uri == "" {
			uri = os.Getenv("MONGO_URL")
		}
		opts = append(
			[]*options.ClientOptions{options.Client().ApplyURI(uri)},
			opts...,
		)

		var err error
		if s.client, err = mongo.Connect(ctx, opts...); err != nil {
			s.client = nil
			return fmt.Errorf("mongo.Connect: %w", err)
		}
	}
	s.db = s.client.Database(s.dbname)
	s.entries = s.db.Collection(s.entriesCol)
	s.states = s.db.Collection(s.statesCol)
	return nil
}

func (s *EventStore) ensureIndexes(ctx context.Context) error {
	type existingIndex struct {
		Name string
	}

	models := append(indices.EventStoreCore(), s.additionalIndices...)
	cur, err := s.entries.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("list existing indexes: %w", err)
	}
	defer cur.Close(ctx)

	indexNames := make(map[string]bool)
	for cur.Next(ctx) {
		var idx existingIndex
		if err := cur.Decode(&idx); err != nil {
			return fmt.Errorf("decode existing index: %w", err)
		}
		indexNames[idx.Name] = true
	}
	if err := cur.Err(); err != nil {
		return fmt.Errorf("cursor error: %w", err)
	}

	for _, model := range models {
		if model.Options == nil {
			continue
		}

		if indexNames[*model.Options.Name] {
			continue
		}

		if _, err := s.entries.Indexes().CreateOne(ctx, model); err != nil {
			return fmt.Errorf("create %q index: %w", *model.Options.Name, err)
		}
	}

	return nil
}

func (e entry) event(enc codec.Encoding) (event.Event, error) {
	data, err := enc.Unmarshal(e.Data, e.Name)
	if err != nil {
		return nil, fmt.Errorf("decode %q event data: %w", e.Name, err)
	}
	return event.New(
		e.Name,
		data,
		event.ID(e.ID),
		event.Time(stdtime.Unix(0, e.TimeNano)),
		event.Aggregate(e.AggregateID, e.AggregateName, e.AggregateVersion),
	), nil
}

func makeFilter(q event.Query) bson.D {
	filter := make(bson.D, 0)
	filter = withIDFilter(filter, q.IDs()...)
	filter = withTimeFilter(filter, q.Times())
	filter = withNameFilter(filter, q.Names()...)
	filter = withAggregateNameFilter(filter, q.AggregateNames()...)
	filter = withAggregateIDFilter(filter, q.AggregateIDs()...)
	filter = withAggregateVersionFilter(filter, q.AggregateVersions())
	filter = withAggregateRefFilter(filter, q.Aggregates())
	return filter
}

func withNameFilter(filter bson.D, names ...string) bson.D {
	if len(names) == 0 {
		return filter
	}
	return append(filter, bson.E{
		Key: "name", Value: bson.D{{Key: "$in", Value: names}},
	})
}

func withIDFilter(filter bson.D, ids ...uuid.UUID) bson.D {
	if len(ids) == 0 {
		return filter
	}
	return append(filter, bson.E{
		Key: "id", Value: bson.D{{Key: "$in", Value: ids}},
	})
}

func withTimeFilter(filter bson.D, times time.Constraints) bson.D {
	if times == nil {
		return filter
	}

	if exact := times.Exact(); len(exact) > 0 {
		filter = append(filter, bson.E{
			Key:   "timeNano",
			Value: bson.D{{Key: "$in", Value: nanoTimes(exact...)}},
		})
	}

	if min := times.Min(); !min.IsZero() {
		filter = append(filter, bson.E{Key: "timeNano", Value: bson.D{{
			Key: "$gte", Value: min.UnixNano(),
		}}})
	}

	if max := times.Max(); !max.IsZero() {
		filter = append(filter, bson.E{Key: "timeNano", Value: bson.D{{
			Key: "$lte", Value: max.UnixNano(),
		}}})
	}

	if ranges := times.Ranges(); len(ranges) > 0 {
		var or []bson.D
		for _, r := range ranges {
			or = append(or, bson.D{{Key: "timeNano", Value: bson.D{
				{Key: "$gte", Value: r.Start().UnixNano()},
				{Key: "$lte", Value: r.End().UnixNano()},
			}}})
		}
		filter = append(filter, bson.E{Key: "$or", Value: or})
	}

	return filter
}

func nanoTimes(ts ...stdtime.Time) []int64 {
	nano := make([]int64, len(ts))
	for i, t := range ts {
		nano[i] = t.UnixNano()
	}
	return nano
}

func withAggregateNameFilter(filter bson.D, names ...string) bson.D {
	if len(names) == 0 {
		return filter
	}
	return append(filter, bson.E{
		Key: "aggregateName", Value: bson.D{{Key: "$in", Value: names}},
	})
}

func withAggregateIDFilter(filter bson.D, ids ...uuid.UUID) bson.D {
	if len(ids) == 0 {
		return filter
	}
	return append(filter, bson.E{
		Key: "aggregateId", Value: bson.D{{Key: "$in", Value: ids}},
	})
}

func withAggregateVersionFilter(filter bson.D, versions version.Constraints) bson.D {
	if versions == nil {
		return filter
	}

	if exact := versions.Exact(); len(exact) > 0 {
		filter = append(filter, bson.E{
			Key: "aggregateVersion", Value: bson.D{{Key: "$in", Value: exact}},
		})
	}

	if min := versions.Min(); len(min) > 0 {
		var or []bson.D
		for _, v := range min {
			or = append(or, bson.D{{
				Key: "aggregateVersion", Value: bson.D{{Key: "$gte", Value: v}},
			}})
		}
		filter = append(filter, bson.E{Key: "$or", Value: or})
	}

	if max := versions.Max(); len(max) > 0 {
		var or []bson.D
		for _, v := range max {
			or = append(or, bson.D{{
				Key: "aggregateVersion", Value: bson.D{{Key: "$lte", Value: v}},
			}})
		}
		filter = append(filter, bson.E{Key: "$or", Value: or})
	}

	if ranges := versions.Ranges(); len(ranges) > 0 {
		var or []bson.D
		for _, r := range ranges {
			or = append(or, bson.D{{Key: "aggregateVersion", Value: bson.D{
				{Key: "$gte", Value: r.Start()},
				{Key: "$lte", Value: r.End()},
			}}})
		}
		filter = append(filter, bson.E{Key: "$or", Value: or})
	}

	return filter
}

func withAggregateRefFilter(filter bson.D, tuples []event.AggregateRef) bson.D {
	if len(tuples) == 0 {
		return filter
	}

	or := make([]bson.D, len(tuples))
	for i, tuple := range tuples {
		f := bson.D{{Key: "aggregateName", Value: tuple.Name}}
		if tuple.ID != uuid.Nil {
			f = append(f, bson.E{Key: "aggregateId", Value: tuple.ID})
		}
		or[i] = f
	}

	return append(filter, bson.E{Key: "$or", Value: or})
}

func applySortings(opts *options.FindOptions, sortings ...event.SortOptions) *options.FindOptions {
	sorts := make(bson.D, len(sortings))
	for i, opts := range sortings {
		v := 1
		if !opts.Dir.Bool(true) {
			v = -1
		}

		switch opts.Sort {
		case event.SortAggregateName:
			sorts[i] = bson.E{Key: "aggregateName", Value: v}
		case event.SortAggregateID:
			sorts[i] = bson.E{Key: "aggregateId", Value: v}
		case event.SortAggregateVersion:
			sorts[i] = bson.E{Key: "aggregateVersion", Value: v}
		case event.SortTime:
			sorts[i] = bson.E{Key: "timeNano", Value: v}
		}
	}
	return opts.SetSort(sorts)
}

func checkDeletion(events []event.Event) (string, uuid.UUID, int, bool) {
	head := events[0]
	tail := events[1:]

	aggregateName := pick.AggregateName(head)
	aggregateID := pick.AggregateID(head)
	aggregateVersion := pick.AggregateVersion(head)

	for _, evt := range tail {
		id, name, v := evt.Aggregate()
		if name != aggregateName || id != aggregateID {
			return aggregateName, aggregateID, aggregateVersion, false
		}

		if v > aggregateVersion {
			aggregateVersion = v
		}
	}

	return aggregateName, aggregateID, aggregateVersion, aggregateName != "" || aggregateID != uuid.Nil || aggregateVersion > 0
}
