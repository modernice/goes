package mongo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	stdtime "time"

	"github.com/modernice/goes"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"github.com/modernice/goes/helper/pick"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EventStore is the MongoDB event.Store[ID]
type EventStore[ID goes.ID] struct {
	eventStoreOptions

	db      *mongo.Database
	entries *mongo.Collection
	states  *mongo.Collection

	onceConnect sync.Once
}

// EventStoreOption is an EventStore option.
type EventStoreOption func(*eventStoreOptions)

type eventStoreOptions struct {
	enc              codec.Encoding
	url              string
	dbname           string
	entriesCol       string
	statesCol        string
	transactions     bool
	validateVersions bool
	client           *mongo.Client
}

// A VersionError means the insertion of events failed because at least one of
// the events has an invalid/inconsistent version.
type VersionError[ID goes.ID] struct {
	// AggregateName is the name of the Aggregate.
	AggregateName string

	// AggregateID is the UUID of the Aggregate.
	AggregateID ID

	// CurrentVersion is the current version of the Aggregate.
	CurrentVersion int

	// Event is the event with the invalid version.
	Event event.Of[any, ID]
}

type state[ID goes.ID] struct {
	AggregateName string `bson:"aggregateName"`
	AggregageID   ID     `bson:"aggregateId"`
	Version       int    `bson:"version"`
}

type entry[ID goes.ID] struct {
	ID               ID           `bson:"id"`
	Name             string       `bson:"name"`
	Time             stdtime.Time `bson:"time"`
	TimeNano         int64        `bson:"timeNano"`
	AggregateName    string       `bson:"aggregateName"`
	AggregateID      ID           `bson:"aggregateId"`
	AggregateVersion int          `bson:"aggregateVersion"`
	Data             []byte       `bson:"data"`
}

// URL returns an Option that specifies the URL to the MongoDB instance. An
// empty URL means "use the default".
//
// Defaults to the environment variable "MONGO_URL".
func URL(url string) EventStoreOption {
	return func(opts *eventStoreOptions) {
		opts.url = url
	}
}

// Client returns an Option that specifies the underlying mongo.Client to be
// used by the Store.
func Client(c *mongo.Client) EventStoreOption {
	return func(opts *eventStoreOptions) {
		opts.client = c
	}
}

// Database returns an Option that sets the mongo database to use for the events.
func Database(name string) EventStoreOption {
	return func(opts *eventStoreOptions) {
		opts.dbname = name
	}
}

// Collection returns an Option that sets the mongo collection where the Events
// are stored in.
func Collection(name string) EventStoreOption {
	return func(opts *eventStoreOptions) {
		opts.entriesCol = name
	}
}

// StateCollection returns an Option that specifies the name of the Collection
// where the current state of Aggregates are stored in.
func StateCollection(name string) EventStoreOption {
	return func(opts *eventStoreOptions) {
		opts.statesCol = name
	}
}

// Transactions returns an Option that, if tx is true, configures a Store to use
// MongoDB Transactions when inserting Events.
//
// Transactions can only be used in replica sets or sharded clusters:
// https://docs.mongodb.com/manual/core/transactions/
func Transactions(tx bool) EventStoreOption {
	return func(opts *eventStoreOptions) {
		opts.transactions = tx
	}
}

// ValidateVersions returns an Option that enables validation of Event versions
// before inserting them into the Store.
//
// Defaults to true.
func ValidateVersions(v bool) EventStoreOption {
	return func(opts *eventStoreOptions) {
		opts.validateVersions = v
	}
}

// NewEventStore returns a MongoDB event.Store[ID]
func NewEventStore[ID goes.ID](enc codec.Encoding, opts ...EventStoreOption) *EventStore[ID] {
	options := eventStoreOptions{
		enc:              enc,
		validateVersions: true,
	}
	for _, opt := range opts {
		opt(&options)
	}

	if strings.TrimSpace(options.dbname) == "" {
		options.dbname = "event"
	}
	if strings.TrimSpace(options.entriesCol) == "" {
		options.entriesCol = "events"
	}
	if strings.TrimSpace(options.statesCol) == "" {
		options.statesCol = "states"
	}

	return &EventStore[ID]{eventStoreOptions: options}
}

// Client returns the underlying mongo.Client. If no mongo.Client is provided
// with the Client option, Client returns nil until the connection to MongoDB
// has been established by either explicitly calling s.Connect or implicitly by
// calling s.Insert, s.Find, s.Delete or s.Query. Otherwise Client returns the
// provided mongo.Client.
func (s *EventStore[ID]) Client() *mongo.Client {
	return s.client
}

// Database returns the underlying mongo.Database. Database returns nil until
// the connection to MongoDB has been established by either explicitly calling
// s.Connect or implicitly by calling s.Insert, s.Find, s.Delete or s.Query.
func (s *EventStore[ID]) Database() *mongo.Database {
	return s.db
}

// Collection returns the underlying *mongo.Collection where the Events are
// stored in. Collection returns nil until the connection to MongoDB has been
// established by either explicitly calling s.Connect or implicitly by calling
// s.Insert, s.Find, s.Delete or s.Query.
func (s *EventStore[ID]) Collection() *mongo.Collection {
	return s.entries
}

// StateCollection returns the underlying *mongo.Collection where Aggregate
// states are stored in. StateCollection returns nil until the connection to
// MongoDB has been established by either explicitly calling s.Connect or
// implicitly by calling s.Insert, s.Find, s.Delete or s.Query.
func (s *EventStore[ID]) StateCollection() *mongo.Collection {
	return s.states
}

// Insert saves the given Events into the database.
func (s *EventStore[ID]) Insert(ctx context.Context, events ...event.Of[any, ID]) error {
	if err := s.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	return s.client.UseSession(ctx, func(ctx mongo.SessionContext) error {
		if s.transactions {
			if err := ctx.StartTransaction(); err != nil {
				return fmt.Errorf("start transaction: %w", err)
			}
		}

		st, err := s.validateEventVersions(ctx, events)
		if err != nil {
			return fmt.Errorf("validate version: %w", err)
		}

		if err := s.insert(ctx, events); err != nil {
			if s.transactions {
				if abortError := ctx.AbortTransaction(ctx); abortError != nil {
					return fmt.Errorf("abort transaction: %w", abortError)
				}
			}
			return err
		}

		if err := s.updateState(ctx, st, events); err != nil {
			if s.transactions {
				if abortError := ctx.AbortTransaction(ctx); abortError != nil {
					return fmt.Errorf("abort transaction: %w", abortError)
				}
			}
			return fmt.Errorf("update state: %w", err)
		}

		if s.transactions {
			if err := ctx.CommitTransaction(ctx); err != nil {
				return fmt.Errorf("commit transaction: %w", err)
			}
		}

		return nil
	})
}

func (s *EventStore[ID]) validateEventVersions(ctx mongo.SessionContext, events []event.Of[any, ID]) (state[ID], error) {
	if len(events) == 0 {
		return state[ID]{}, nil
	}

	aggregateID, aggregateName, aggregateVersion := events[0].Aggregate()

	var zero ID

	if aggregateName == "" || aggregateID == zero {
		return state[ID]{}, nil
	}

	res := s.states.FindOne(ctx, bson.D{
		{Key: "aggregateName", Value: aggregateName},
		{Key: "aggregateId", Value: aggregateID},
	})

	var st state[ID]
	if err := res.Decode(&st); err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return state[ID]{}, fmt.Errorf("decode state: %w", err)
		}
		st = state[ID]{
			AggregateName: aggregateName,
			AggregageID:   aggregateID,
		}
	}

	if !s.validateVersions {
		return st, nil
	}

	if st.Version >= aggregateVersion {
		return st, &VersionError[ID]{
			AggregateName:  aggregateName,
			AggregateID:    aggregateID,
			CurrentVersion: st.Version,
			Event:          events[0],
		}
	}

	return st, nil
}

func (s *EventStore[ID]) updateState(ctx mongo.SessionContext, st state[ID], events []event.Of[any, ID]) error {
	var zero ID
	if len(events) == 0 || st.AggregateName == "" || st.AggregageID == zero {
		return nil
	}
	st.Version = pick.AggregateVersion[ID](events[len(events)-1])
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

func (s *EventStore[ID]) insert(ctx context.Context, events []event.Of[any, ID]) error {
	if len(events) == 0 {
		return nil
	}

	docs := make([]any, len(events))
	for i, evt := range events {
		var data bytes.Buffer
		if err := s.enc.Encode(&data, evt.Name(), evt.Data()); err != nil {
			return fmt.Errorf("encode %q event data: %w", evt.Name(), err)
		}
		id, name, v := evt.Aggregate()
		docs[i] = entry[ID]{
			ID:               evt.ID(),
			Name:             evt.Name(),
			Time:             evt.Time(),
			TimeNano:         int64(evt.Time().UnixNano()),
			AggregateName:    name,
			AggregateID:      id,
			AggregateVersion: v,
			Data:             data.Bytes(),
		}
	}
	if _, err := s.entries.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	return nil
}

// Find returns the Event with the specified UUID from the database if it exists.
func (s *EventStore[ID]) Find(ctx context.Context, id ID) (event.Of[any, ID], error) {
	if err := s.connectOnce(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	res := s.entries.FindOne(ctx, bson.M{"id": id})
	var e entry[ID]
	if err := res.Decode(&e); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}
	return e.event(s.enc)
}

// Delete deletes the given Event from the database.
func (s *EventStore[ID]) Delete(ctx context.Context, events ...event.Of[any, ID]) error {
	if len(events) == 0 {
		return nil
	}

	ids := make([]ID, len(events))
	for i, evt := range events {
		ids[i] = evt.ID()
	}

	if err := s.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	var zero ID

	return s.client.UseSession(ctx, func(ctx mongo.SessionContext) error {
		if s.transactions {
			if err := ctx.StartTransaction(); err != nil {
				return fmt.Errorf("start transaction: %w", err)
			}
		}

		abort := func(err error) error {
			if s.transactions {
				if abortError := ctx.AbortTransaction(ctx); abortError != nil {
					return fmt.Errorf("abort transaction: %w", abortError)
				}
			}
			return err
		}

		commit := func() error {
			if s.transactions {
				if err := ctx.CommitTransaction(ctx); err != nil {
					return fmt.Errorf("commit transaction: %w", err)
				}
			}
			return nil
		}

		if _, err := s.entries.DeleteMany(ctx, bson.D{
			{Key: "id", Value: bson.D{{Key: "$in", Value: ids}}},
		}); err != nil {
			return abort(err)
		}

		aggregateName := pick.AggregateName[ID](events[0])
		aggregateID := pick.AggregateID[ID](events[0])
		aggregateVersion := pick.AggregateVersion[ID](events[0])

		if aggregateName == "" || aggregateID == zero || aggregateVersion != 1 {
			return commit()
		}

		for _, evt := range events[1:] {
			id, name, v := evt.Aggregate()
			if name != aggregateName || id != aggregateID {
				return commit()
			}

			if v > aggregateVersion {
				aggregateVersion = v
			}
		}

		if _, err := s.states.DeleteOne(ctx, bson.D{
			{Key: "aggregateName", Value: aggregateName},
			{Key: "aggregateId", Value: aggregateID},
			{Key: "aggregateVersion", Value: aggregateVersion},
		}); err != nil {
			return abort(fmt.Errorf("delete aggregate state: %w", err))
		}

		return commit()
	})
}

// Query queries the database for events filtered by Query q and returns an
// streams.New for those events.
func (s *EventStore[ID]) Query(ctx context.Context, q event.Query) (<-chan event.Of[any, ID], <-chan error, error) {
	if err := s.connectOnce(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}
	opts := options.Find()
	opts = applySortings(opts, q.Sortings()...)

	f := makeFilter(q)

	cur, err := s.entries.Find(ctx, f, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("mongo: %w", err)
	}

	events := make(chan event.Of[any, ID])
	errs := make(chan error)

	go func() {
		defer close(events)
		defer close(errs)

	L:
		for cur.Next(ctx) {
			var e entry[ID]
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
func (s *EventStore[ID]) Connect(ctx context.Context, opts ...*options.ClientOptions) (*mongo.Client, error) {
	if err := s.connectOnce(ctx, opts...); err != nil {
		return nil, err
	}
	return s.client, nil
}

func (s *EventStore[ID]) connectOnce(ctx context.Context, opts ...*options.ClientOptions) error {
	var err error
	s.onceConnect.Do(func() {
		if err = s.connect(ctx, opts...); err != nil {
			return
		}
		if err = s.ensureIndexes(ctx); err != nil {
			err = fmt.Errorf("ensure indexes: %w", err)
			return
		}
	})
	return err
}

func (s *EventStore[ID]) connect(ctx context.Context, opts ...*options.ClientOptions) error {
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

func (s *EventStore[ID]) ensureIndexes(ctx context.Context) error {
	// TODO: make indexes configurable

	var zero ID

	if _, err := s.entries.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetName("goes_id").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetName("goes_name"),
		},
		{
			Keys:    bson.D{{Key: "time", Value: 1}},
			Options: options.Index().SetName("goes_time"),
		},
		{
			Keys:    bson.D{{Key: "timeNano", Value: 1}},
			Options: options.Index().SetName("goes_time_nano"),
		},
		{
			Keys:    bson.D{{Key: "aggregateName", Value: 1}},
			Options: options.Index().SetName("goes_aggregate_name"),
		},
		{
			Keys:    bson.D{{Key: "aggregateId", Value: 1}},
			Options: options.Index().SetName("goes_aggregate_id"),
		},
		{
			Keys:    bson.D{{Key: "aggregateVersion", Value: 1}},
			Options: options.Index().SetName("goes_aggregate_version"),
		},
		{
			Keys: bson.D{
				{Key: "aggregateName", Value: 1},
				{Key: "aggregateId", Value: 1},
				{Key: "aggregateVersion", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(
				bson.D{
					{Key: "aggregateName", Value: bson.D{{Key: "$gt", Value: ""}}},
					{Key: "aggregateId", Value: bson.D{{Key: "$gt", Value: zero}}},
					{Key: "aggregateVersion", Value: bson.D{{Key: "$gt", Value: 0}}},
				},
			),
		},
	}); err != nil {
		return fmt.Errorf("create indexes (%s): %w", s.entries.Name(), err)
	}

	return nil
}

func (e entry[ID]) event(enc codec.Encoding) (event.Of[any, ID], error) {
	data, err := enc.Decode(bytes.NewReader(e.Data), e.Name)
	if err != nil {
		return nil, fmt.Errorf("decode %q event data: %w", e.Name, err)
	}
	return event.New(
		e.ID,
		e.Name,
		data,
		event.Time(stdtime.Unix(0, e.TimeNano)),
		event.Aggregate(e.AggregateID, e.AggregateName, e.AggregateVersion),
	), nil
}

func (err *VersionError[ID]) Error() string {
	return fmt.Sprintf(
		"event should have version %d, but has version %d",
		err.CurrentVersion+1,
		pick.AggregateVersion[ID](err.Event),
	)
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

func withIDFilter[ID goes.ID](filter bson.D, ids ...ID) bson.D {
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

func withAggregateIDFilter[ID goes.ID](filter bson.D, ids ...ID) bson.D {
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

func withAggregateRefFilter[ID goes.ID](filter bson.D, tuples []event.AggregateRefOf[ID]) bson.D {
	if len(tuples) == 0 {
		return filter
	}

	var zero ID

	or := make([]bson.D, len(tuples))
	for i, tuple := range tuples {
		f := bson.D{{Key: "aggregateName", Value: tuple.Name}}
		if tuple.ID != zero {
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
