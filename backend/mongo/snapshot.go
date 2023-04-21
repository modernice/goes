package mongo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	stdtime "time"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/aggregate/snapshot"
	"github.com/modernice/goes/event/query/time"
	"github.com/modernice/goes/event/query/version"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	// ErrNotFound is returned when a snapshot can't be found in the database.
	ErrNotFound = errors.New("snapshot not found")
)

// SnapshotStore is the MongoDB implementation of a snapshot store.
type SnapshotStore struct {
	url     string
	dbname  string
	colname string

	client *mongo.Client
	db     *mongo.Database
	col    *mongo.Collection

	onceConnect sync.Once
}

// Option is a Store option.
type Option func(*SnapshotStore)

type snapshotEntry struct {
	AggregateName    string       `bson:"aggregateName"`
	AggregateID      uuid.UUID    `bson:"aggregateId"`
	AggregateVersion int          `bson:"aggregateVersion"`
	Time             stdtime.Time `bson:"time"`
	TimeNano         int64        `bson:"timeNano"`
	Data             []byte       `bson:"data"`
}

// SnapshotURL returns an Option that specifies the URL to the MongoDB instance. An
// empty SnapshotURL means "use the default".
//
// Defaults to the environment variable "MONGO_URL".
func SnapshotURL(url string) Option {
	return func(s *SnapshotStore) {
		s.url = url
	}
}

// SnapshotDatabase returns an Option that specifies the database name for snapshots.
func SnapshotDatabase(name string) Option {
	return func(s *SnapshotStore) {
		s.dbname = name
	}
}

// SnapshotCollection returns an Option that specifies the collection name for
// Snapshots.
func SnapshotCollection(name string) Option {
	return func(s *SnapshotStore) {
		s.colname = name
	}
}

// NewSnapshotStore returns a new Store.
func NewSnapshotStore(opts ...Option) *SnapshotStore {
	var s SnapshotStore
	for _, opt := range opts {
		opt(&s)
	}
	if s.dbname == "" {
		s.dbname = "snapshot"
	}
	if s.colname == "" {
		s.colname = "snapshots"
	}
	return &s
}

// Save saves the given Snapshot into the database.
func (s *SnapshotStore) Save(ctx context.Context, snap snapshot.Snapshot) error {
	if err := s.connectOnce(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	e := snapshotEntry{
		AggregateName:    snap.AggregateName(),
		AggregateID:      snap.AggregateID(),
		AggregateVersion: snap.AggregateVersion(),
		Time:             snap.Time(),
		TimeNano:         snap.Time().UnixNano(),
		Data:             snap.State(),
	}

	if _, err := s.col.ReplaceOne(ctx, bson.D{
		{Key: "aggregateName", Value: snap.AggregateName()},
		{Key: "aggregateId", Value: snap.AggregateID()},
		{Key: "aggregateVersion", Value: snap.AggregateVersion()},
	}, e, options.Replace().SetUpsert(true)); err != nil {
		return fmt.Errorf("mongo: %w", err)
	}

	return nil
}

// Latest returns the latest Snapshot for the aggregate with the given name and
// UUID or ErrNotFound if no Snapshots for that aggregate exist in the database.
func (s *SnapshotStore) Latest(ctx context.Context, name string, id uuid.UUID) (snapshot.Snapshot, error) {
	if err := s.connectOnce(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	res := s.col.FindOne(ctx, bson.D{
		{Key: "aggregateName", Value: name},
		{Key: "aggregateId", Value: id},
	}, options.FindOne().SetSort(bson.D{
		{Key: "aggregateVersion", Value: -1},
	}))

	var e snapshotEntry
	if err := res.Decode(&e); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mongo: decode result: %w", err)
	}

	return e.snapshot()
}

// Version returns the Snapshot for the aggregate with the given name, UUID and
// version. If no Snapshot for the given version exists, Version returns
// ErrNotFound.
func (s *SnapshotStore) Version(ctx context.Context, name string, id uuid.UUID, version int) (snapshot.Snapshot, error) {
	if err := s.connectOnce(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	res := s.col.FindOne(ctx, bson.D{
		{Key: "aggregateName", Value: name},
		{Key: "aggregateId", Value: id},
		{Key: "aggregateVersion", Value: version},
	})

	var e snapshotEntry
	if err := res.Decode(&e); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mongo: decode result: %w", err)
	}

	return e.snapshot()
}

// Limit returns the latest Snapshot that has a version equal to or lower
// than the given version.
//
// Limit returns ErrNotFound if no such Snapshot can be found in the database.
func (s *SnapshotStore) Limit(ctx context.Context, name string, id uuid.UUID, v int) (snapshot.Snapshot, error) {
	res := s.col.FindOne(
		ctx,
		bson.D{
			{Key: "aggregateName", Value: name},
			{Key: "aggregateId", Value: id},
			{Key: "aggregateVersion", Value: bson.D{
				{Key: "$lte", Value: v},
			}},
		},
		options.FindOne().SetSort(bson.D{
			{Key: "aggregateVersion", Value: -1},
		}),
	)

	var e snapshotEntry
	if err := res.Decode(&e); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mongo: decode result: %w", err)
	}

	return e.snapshot()
}

// Query returns a channel of Snapshots and a channel of errors. It takes a
// context.Context and a snapshot.Query as parameters. The returned Snapshots
// match the criteria defined by the snapshot.Query. The order in which
// Snapshots are returned is determined by the Sortings defined in the
// snapshot.Query. The channels are closed when there are no more Snapshots or
// an error occurs, respectively.
func (s *SnapshotStore) Query(ctx context.Context, q snapshot.Query) (<-chan snapshot.Snapshot, <-chan error, error) {
	filter := makeSnapshotFilter(q)
	opts := options.Find()
	applySnapshotSortings(opts, q.Sortings()...)
	cur, err := s.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("mongo: %w", err)
	}

	out, outErrs := make(chan snapshot.Snapshot), make(chan error)

	go func() {
		defer close(out)
		defer close(outErrs)

		for cur.Next(ctx) {
			var e snapshotEntry
			if err = cur.Decode(&e); err != nil {
				outErrs <- fmt.Errorf("decode mongo result: %w", err)
				continue
			}

			snap, err := e.snapshot()
			if err != nil {
				outErrs <- err
				continue
			}
			out <- snap
		}

		if cur.Err() != nil {
			outErrs <- fmt.Errorf("mongo cursor: %w", err)
		}

		if err := cur.Close(ctx); err != nil {
			outErrs <- fmt.Errorf("close mongo cursor: %w", err)
		}
	}()

	return out, outErrs, nil
}

// Delete deletes a Snapshot from the database.
func (s *SnapshotStore) Delete(ctx context.Context, snap snapshot.Snapshot) error {
	if _, err := s.col.DeleteOne(ctx, bson.D{
		{Key: "aggregateName", Value: snap.AggregateName()},
		{Key: "aggregateId", Value: snap.AggregateID()},
		{Key: "aggregateVersion", Value: snap.AggregateVersion()},
	}); err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	return nil
}

// Connect establishes the connection to the underlying MongoDB and returns the
// mongo.Client. Connect doesn't need to be called manually as it's called
// automatically on the first call to s.Save, s.Latest, s.Version, s.Query or
// s.Delete. Use Connect if you want to explicitly control when to connect to
// MongoDB.
func (s *SnapshotStore) Connect(ctx context.Context) (*mongo.Client, error) {
	if err := s.connectOnce(ctx); err != nil {
		return nil, err
	}
	return s.client, nil
}

func (s *SnapshotStore) connectOnce(ctx context.Context) error {
	var err error
	s.onceConnect.Do(func() {
		if err = s.connect(ctx); err != nil {
			return
		}
		if err = s.ensureIndexes(ctx); err != nil {
			err = fmt.Errorf("ensure indexes: %w", err)
			return
		}
	})
	return err
}

func (s *SnapshotStore) connect(ctx context.Context) error {
	uri := s.url
	if uri == "" {
		uri = os.Getenv("MONGO_URL")
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	db := client.Database(s.dbname)
	col := db.Collection(s.colname)
	s.client = client
	s.db = db
	s.col = col
	return nil
}

func (s *SnapshotStore) ensureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "time", Value: -1}},
			Options: options.Index().SetName("goes_time"),
		},
		{
			Keys:    bson.D{{Key: "timeNano", Value: -1}},
			Options: options.Index().SetName("goes_time_nano"),
		},
		{
			Keys: bson.D{
				{Key: "aggregateName", Value: 1},
				{Key: "aggregateId", Value: 1},
				{Key: "aggregateVersion", Value: -1},
			},
			Options: options.Index().
				SetName("goes_aggregate").
				SetUnique(true),
		},
	})
	return err
}

func makeSnapshotFilter(q snapshot.Query) bson.D {
	filter := make(bson.D, 0)
	filter = withSnapshotNameFilter(filter, q.Names())
	filter = withSnapshotIDFilter(filter, q.IDs())
	filter = withSnapshotVersionFilter(filter, q.Versions())
	filter = withSnapshotTimeFilter(filter, q.Times())
	return filter
}

func withSnapshotNameFilter(filter bson.D, names []string) bson.D {
	if len(names) == 0 {
		return filter
	}
	return append(filter, bson.E{
		Key:   "aggregateName",
		Value: bson.D{{Key: "$in", Value: names}},
	})
}

func withSnapshotIDFilter(filter bson.D, ids []uuid.UUID) bson.D {
	if len(ids) == 0 {
		return filter
	}
	return append(filter, bson.E{
		Key:   "aggregateId",
		Value: bson.D{{Key: "$in", Value: ids}},
	})
}

func withSnapshotVersionFilter(filter bson.D, versions version.Constraints) bson.D {
	if exact := versions.Exact(); len(exact) > 0 {
		filter = append(filter, bson.E{Key: "aggregateVersion", Value: bson.D{
			{Key: "$in", Value: exact},
		}})
	}
	if min := versions.Min(); len(min) > 0 {
		filter = append(filter, bson.E{Key: "aggregateVersion", Value: bson.D{
			{Key: "$gte", Value: min},
		}})
	}
	if max := versions.Max(); len(max) > 0 {
		filter = append(filter, bson.E{Key: "aggregateVersion", Value: bson.D{
			{Key: "$lte", Value: max},
		}})
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

func withSnapshotTimeFilter(filter bson.D, times time.Constraints) bson.D {
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

func applySnapshotSortings(opts *options.FindOptions, sortings ...aggregate.SortOptions) *options.FindOptions {
	sorts := make(bson.D, len(sortings))
	for i, opts := range sortings {
		v := 1
		if !opts.Dir.Bool(true) {
			v = -1
		}

		switch opts.Sort {
		case aggregate.SortName:
			sorts[i] = bson.E{Key: "aggregateName", Value: v}
		case aggregate.SortID:
			sorts[i] = bson.E{Key: "aggregateId", Value: v}
		case aggregate.SortVersion:
			sorts[i] = bson.E{Key: "aggregateVersion", Value: v}
		}
	}
	return opts.SetSort(sorts)
}

func (e snapshotEntry) snapshot() (snapshot.Snapshot, error) {
	return snapshot.New(
		aggregate.New(
			e.AggregateName,
			e.AggregateID,
			aggregate.Version(e.AggregateVersion),
		),
		snapshot.Time(stdtime.Unix(0, e.TimeNano)),
		snapshot.Data(e.Data),
	)
}
