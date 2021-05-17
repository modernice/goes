package mongotags

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate/tagging"
	"github.com/modernice/goes/event"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Option is a Store option.
type Option func(*Store)

type Store struct {
	enc event.Encoder

	transactions bool
	url          string
	dbname       string
	colname      string

	onceConnect sync.Once
	client      *mongo.Client
	db          *mongo.Database
	entries     *mongo.Collection
}

type entry struct {
	AggregateName string    `bson:"aggregateName"`
	AggregateID   uuid.UUID `bson:"aggregateId"`
	Tags          []string  `bson:"tags"`
}

// URL returns an Option that specifies the URL to the MongoDB instance. An
// empty URL means "use the default".
//
// Defaults to the environment variable "MONGO_URL".
func URL(url string) Option {
	return func(s *Store) {
		s.url = url
	}
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

// Collection returns an Option that sets the mongo collection where the tags
// are stored in.
func Collection(name string) Option {
	return func(s *Store) {
		s.colname = name
	}
}

// Transactions returns an Option that, if tx is true, configures a Store to use
// MongoDB Transactions when inserting Events.
//
// Transactions can only be used in replica sets or sharded clusters:
// https://docs.mongodb.com/manual/core/transactions/
func Transactions(tx bool) Option {
	return func(s *Store) {
		s.transactions = tx
	}
}

func NewStore(enc event.Encoder, opts ...Option) *Store {
	s := Store{enc: enc}
	for _, opt := range opts {
		opt(&s)
	}
	if strings.TrimSpace(s.dbname) == "" {
		s.dbname = "tagging"
	}
	if strings.TrimSpace(s.colname) == "" {
		s.colname = "tags"
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

// Collection returns the underlying *mongo.Collection where the tags are
// stored in. Collection returns nil until the connection to MongoDB has been
// established by either explicitly calling s.Connect or implicitly by calling
// s.Update, s.Tags, s.TaggedWith.
func (s *Store) Collection() *mongo.Collection {
	return s.entries
}

// Connect establishes the connection to the underlying MongoDB and returns the
// mongo.Client. Connect doesn't need to be called manually as it's called
// automatically on the first call to s.Insert, s.Find, s.Delete or s.Query. Use
// Connect if you want to explicitly control when to connect to MongoDB.
func (s *Store) Connect(ctx context.Context, opts ...*options.ClientOptions) (*mongo.Client, error) {
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
	return s.client, err
}

func (s *Store) connect(ctx context.Context, opts ...*options.ClientOptions) error {
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
	s.entries = s.db.Collection(s.colname)
	return nil
}

func (s *Store) ensureIndexes(ctx context.Context) error {
	if _, err := s.entries.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "aggregateName", Value: 1},
				{Key: "aggregateId", Value: 1},
			},
			Options: options.Index().SetName("goes_aggregate").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "tags", Value: 1}},
			Options: options.Index().SetName("goes_tags"),
		},
	}); err != nil {
		return fmt.Errorf("create indexes (%s): %w", s.entries.Name(), err)
	}

	return nil
}

func (s *Store) Update(ctx context.Context, name string, id uuid.UUID, tags []string) error {
	if _, err := s.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	if err := s.client.UseSession(ctx, func(ctx mongo.SessionContext) error {
		if s.transactions {
			if err := ctx.StartTransaction(); err != nil {
				return fmt.Errorf("start transaction: %w", err)
			}
		}

		if len(tags) > 0 {
			if _, err := s.entries.ReplaceOne(ctx, bson.D{
				{Key: "aggregateName", Value: name},
				{Key: "aggregateId", Value: id},
			}, bson.D{
				{Key: "aggregateName", Value: name},
				{Key: "aggregateId", Value: id},
				{Key: "tags", Value: tags},
			}, options.Replace().SetUpsert(true)); err != nil {
				return fmt.Errorf("replace entry: %w", err)
			}
		} else {
			if _, err := s.entries.DeleteOne(ctx, bson.D{
				{Key: "aggregateName", Value: name},
				{Key: "aggregateId", Value: id},
			}); err != nil {
				return fmt.Errorf("delete entry: %w", err)
			}
		}

		if s.transactions {
			if err := ctx.CommitTransaction(ctx); err != nil {
				return fmt.Errorf("commit transaction: %w", err)
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *Store) Tags(ctx context.Context, name string, id uuid.UUID) ([]string, error) {
	if _, err := s.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	res := s.entries.FindOne(ctx, bson.D{
		{Key: "aggregateName", Value: name},
		{Key: "aggregateId", Value: id},
	})

	var e entry
	if err := res.Decode(&e); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("mongo: %w", err)
	}

	return e.Tags, nil
}

func (s *Store) TaggedWith(ctx context.Context, tags ...string) ([]tagging.Aggregate, error) {
	if _, err := s.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	filter := make(bson.D, 0)
	if len(tags) > 0 {
		filter = bson.D{{Key: "tags", Value: bson.D{{Key: "$all", Value: tags}}}}
	}
	cur, err := s.entries.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("find entries: %w", err)
	}
	defer cur.Close(ctx)

	var out []tagging.Aggregate
	for cur.Next(ctx) {
		var e entry
		if err := cur.Decode(&e); err != nil {
			return out, fmt.Errorf("decode entry: %w", err)
		}
		out = append(out, tagging.Aggregate{
			Name: e.AggregateName,
			ID:   e.AggregateID,
		})
	}

	if err := cur.Err(); err != nil {
		return out, fmt.Errorf("cursor: %w", err)
	}

	return out, nil
}
