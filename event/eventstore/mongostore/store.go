// Package mongostore provides a MongoDB event.Store.
package mongostore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store is the MongoDB event.Store.
type Store struct {
	enc     event.Encoder
	dbname  string
	colname string

	client *mongo.Client
	db     *mongo.Database
	col    *mongo.Collection

	onceConnect sync.Once
}

// Option is a Store option.
type Option func(*Store)

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

type stream struct {
	enc event.Encoder
	cur *mongo.Cursor
	evt event.Event
	err error
}

// Client returns an Option that specifies the underlying mongo.Client to be
// used by the Store.
func Client(c *mongo.Client) Option {
	return func(s *Store) {
		s.client = c
	}
}

// Database returns an Option that sets the mongo database to use for the events.
func Database(name string) Option {
	return func(s *Store) {
		s.dbname = name
	}
}

// Collection returns an Option that sets the mongo collection where the Events
// are stored in.
func Collection(name string) Option {
	return func(s *Store) {
		s.colname = name
	}
}

// New returns a MongoDB event.Store.
func New(enc event.Encoder, opts ...Option) *Store {
	s := Store{enc: enc}
	for _, opt := range opts {
		opt(&s)
	}
	if strings.TrimSpace(s.dbname) == "" {
		s.dbname = "event"
	}
	if strings.TrimSpace(s.colname) == "" {
		s.colname = "events"
	}
	return &s
}

// Client returns the underlying mongo.Client. If no mongo.Client is provided
// with the Client option, Client returns nil until the connection to MongoDB
// has been established by either explicitly calling s.Connect or implicitly by
// calling s.Insert, s.Find, s.Delete or s.Query. Otherwise Client returns the
// provided mongo.Client.
func (s *Store) Client() *mongo.Client {
	return s.client
}

// Database returns the underlying mongo.Database. Database returns nil until
// the connection to MongoDB has been established by either explicitly calling
// s.Connect or implicitly by calling s.Insert, s.Find, s.Delete or s.Query.
func (s *Store) Database() *mongo.Database {
	return s.db
}

// Collection returns the underlying mongo.Collection. Connection returns nil
// until the connection to MongoDB has been established by either explicitly
// calling s.Connect or implicitly by calling s.Insert, s.Find, s.Delete or
// s.Query.
func (s *Store) Collection() *mongo.Collection {
	return s.col
}

// Insert saves the given Events into the database.
func (s *Store) Insert(ctx context.Context, events ...event.Event) error {
	if err := s.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	for _, evt := range events {
		if err := s.insert(ctx, evt); err != nil {
			return fmt.Errorf("%s:%s: %w", evt.Name(), evt.ID(), err)
		}
	}

	return nil
}

func (s *Store) insert(ctx context.Context, evt event.Event) error {
	var data bytes.Buffer
	if err := s.enc.Encode(&data, evt.Data()); err != nil {
		return fmt.Errorf("encode %q event data: %w", evt.Name(), err)
	}
	if _, err := s.col.InsertOne(ctx, entry{
		ID:               evt.ID(),
		Name:             evt.Name(),
		Time:             evt.Time(),
		TimeNano:         int64(evt.Time().UnixNano()),
		AggregateName:    evt.AggregateName(),
		AggregateID:      evt.AggregateID(),
		AggregateVersion: evt.AggregateVersion(),
		Data:             data.Bytes(),
	}); err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	return nil
}

// Find returns the Event with the specified UUID from the database if it exists.
func (s *Store) Find(ctx context.Context, id uuid.UUID) (event.Event, error) {
	if err := s.connectOnce(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	res := s.col.FindOne(ctx, bson.M{"id": id})
	var e entry
	if err := res.Decode(&e); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}
	return e.event(s.enc)
}

// Delete deletes the given Event from the database.
func (s *Store) Delete(ctx context.Context, evt event.Event) error {
	if err := s.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if _, err := s.col.DeleteOne(ctx, bson.M{"id": evt.ID()}); err != nil {
		return fmt.Errorf("delete event %q: %w", evt.ID(), err)
	}
	return nil
}

// Query queries the database for events filtered by Query q and returns an
// event.Stream for those events.
func (s *Store) Query(ctx context.Context, q event.Query) (event.Stream, error) {
	if err := s.connectOnce(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	opts := options.Find()
	opts = applySortings(opts, q.Sortings()...)

	cur, err := s.col.Find(ctx, makeFilter(q), opts)
	if err != nil {
		return nil, fmt.Errorf("mongo: %w", err)
	}
	return &stream{
		enc: s.enc,
		cur: cur,
	}, nil
}

// Connect establishes the connection to the underlying MongoDB and returns the
// mongo.Client. Connect doesn't need to be called manually as it's called
// automatically on the first call to s.Insert, s.Find, s.Delete or s.Query. Use
// Connect if you want to explicitly control when to connect to MongoDB.
func (s *Store) Connect(ctx context.Context, opts ...*options.ClientOptions) (*mongo.Client, error) {
	if err := s.connectOnce(ctx, opts...); err != nil {
		return nil, err
	}
	return s.client, nil
}

func (s *Store) connectOnce(ctx context.Context, opts ...*options.ClientOptions) error {
	var err error
	s.onceConnect.Do(func() {
		if err = s.connect(ctx, opts...); err != nil {
			return
		}
		if err = s.ensureIndexes(ctx); err != nil {
			return
		}
	})
	return err
}

func (s *Store) connect(ctx context.Context, opts ...*options.ClientOptions) error {
	if s.client == nil {
		uri := os.Getenv("MONGO_URL")
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
	s.col = s.db.Collection(s.colname)
	return nil
}

func (s *Store) ensureIndexes(ctx context.Context) error {
	res, err := s.col.Find(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("find: %w", err)
	}
	var entries []entry
	if err := res.All(ctx, &entries); err != nil {
		return fmt.Errorf("cursor: %w", err)
	}

	if _, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
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
			Keys: bson.D{
				{Key: "aggregateName", Value: 1},
				{Key: "aggregateId", Value: 1},
				{Key: "aggregateVersion", Value: 1},
			},
			Options: options.Index().
				SetName("goes_aggregate").
				SetUnique(true).
				SetPartialFilterExpression(bson.D{
					{Key: "aggregateName", Value: bson.D{
						{Key: "$gt", Value: ""},
					}},
					{Key: "aggregateId", Value: bson.D{
						{Key: "$gt", Value: uuid.Nil},
					}},
				}),
		},
	}); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

func (e entry) event(enc event.Encoder) (event.Event, error) {
	data, err := enc.Decode(e.Name, bytes.NewReader(e.Data))
	if err != nil {
		return nil, fmt.Errorf("decode %q event data: %w", e.Name, err)
	}
	return event.New(
		e.Name,
		data,
		event.ID(e.ID),
		event.Time(stdtime.Unix(0, e.TimeNano)),
		event.Aggregate(e.AggregateName, e.AggregateID, e.AggregateVersion),
	), nil
}

func (c *stream) Next(ctx context.Context) bool {
	if !c.cur.Next(ctx) {
		c.err = c.cur.Err()
		return false
	}

	c.evt = nil
	var e entry
	if c.err = c.cur.Decode(&e); c.err != nil {
		return false
	}

	if c.evt, c.err = e.event(c.enc); c.err != nil {
		return false
	}

	return true
}

func (c *stream) Event() event.Event {
	return c.evt
}

func (c *stream) Err() error {
	return c.err
}

func (c *stream) Close(ctx context.Context) error {
	return c.cur.Close(ctx)
}

func makeFilter(q event.Query) bson.D {
	filter := make(bson.D, 0)
	filter = withNameFilter(filter, q.Names()...)
	filter = withIDFilter(filter, q.IDs()...)
	filter = withTimeFilter(filter, q.Times())
	filter = withAggregateNameFilter(filter, q.AggregateNames()...)
	filter = withAggregateIDFilter(filter, q.AggregateIDs()...)
	filter = withAggregateVersionFilter(filter, q.AggregateVersions())
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
				{Key: "$gte", Value: r.Start()},
				{Key: "$lte", Value: r.End()},
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

func applySortings(opts *options.FindOptions, sortings ...event.SortOptions) *options.FindOptions {
	sorts := make(bson.D, len(sortings))
	for i, opts := range sortings {
		v := 1
		if !opts.Dir.Bool(true) {
			v = -1
		}

		switch opts.Sort {
		case event.SortTime:
			sorts[i] = bson.E{Key: "timeNano", Value: v}
		case event.SortAggregateName:
			sorts[i] = bson.E{Key: "aggregateName", Value: v}
		case event.SortAggregateID:
			sorts[i] = bson.E{Key: "aggregateId", Value: v}
		case event.SortAggregateVersion:
			sorts[i] = bson.E{Key: "aggregateVersion", Value: v}
		}
	}
	return opts.SetSort(sorts)
}
