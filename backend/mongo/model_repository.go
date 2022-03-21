package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/modernice/goes/persistence/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ model.Repository[model.Model[uuid.UUID], uuid.UUID] = &ModelRepository[model.Model[uuid.UUID], uuid.UUID]{}

// ModelRepository is a MongoDB backed model repository.
type ModelRepository[Model model.Model[ID], ID model.ID] struct {
	modelRepositoryOptions
	col *mongo.Collection
}

// ModelRepositoryOption is an option for the model repository.
type ModelRepositoryOption func(*modelRepositoryOptions)

type modelRepositoryOptions struct {
	key              string
	transactions     bool
	factory          func(any) any
	createIfNotFound bool
	customDecoder    func(*mongo.SingleResult, any) error
	customEncoder    func(any) (any, error)
}

// ModelIDKey returns a ModelRepositoryOption that specifies which field of the
// model is the id of the model.
func ModelIDKey(key string) ModelRepositoryOption {
	return func(o *modelRepositoryOptions) {
		o.key = key
	}
}

// ModelTransactions returns a ModelRepositoryOption that enables MongoDB
// transactions for the repository. Currently, only the Use() function makes use
// of transactions. Transactions are disabled by default and must be supported
// by your MongoDB cluster.
func ModelTransactions(tx bool) ModelRepositoryOption {
	return func(o *modelRepositoryOptions) {
		o.transactions = tx
	}
}

// ModelDecoder returns a ModelRepositoryOption that specifies a custom decoder
// for the model.
func ModelDecoder[Model model.Model[ID], ID model.ID](decode func(*mongo.SingleResult, *Model) error) ModelRepositoryOption {
	return func(o *modelRepositoryOptions) {
		o.customDecoder = func(res *mongo.SingleResult, m any) error {
			return decode(res, m.(*Model))
		}
	}
}

// ModelEncoder returns a ModelRepositoryOption that specifies a custom encoder
// for the model. Then a model is saved, the document that is returned by the
// provided encode function is used as the replacement document.
func ModelEncoder[Model model.Model[ID], ID model.ID](encode func(Model) (any, error)) ModelRepositoryOption {
	return func(o *modelRepositoryOptions) {
		o.customEncoder = func(m any) (any, error) {
			return encode(m.(Model))
		}
	}
}

// ModelFactory returns a ModelRepositoryOption that provides a factory function
// for the models to a model repository. The repository will use the function to
// create the model before decoding the MongoDB document into it. Without a
// model factory, the repository will just use the zero value of the provided
// model type. If `createIfNotFound` is true, the repository will create and
// return the model using the factory function instead of returning a
// model.ErrNotFound error.
func ModelFactory[Model model.Model[ID], ID model.ID](factory func(ID) Model, createIfNotFound bool) ModelRepositoryOption {
	return func(o *modelRepositoryOptions) {
		o.createIfNotFound = createIfNotFound
		o.factory = func(id any) any {
			return factory(id.(ID))
		}
	}
}

// NewModelRepository returns a MongoDB backed model repository.
func NewModelRepository[Model model.Model[ID], ID model.ID](col *mongo.Collection, opts ...ModelRepositoryOption) *ModelRepository[Model, ID] {
	var options modelRepositoryOptions
	for _, opt := range opts {
		opt(&options)
	}

	if options.key == "" {
		options.key = "_id"
	}

	return &ModelRepository[Model, ID]{
		modelRepositoryOptions: options,
		col:                    col,
	}
}

// Collection returns the MongoDB collection of the model.
func (r *ModelRepository[Model, ID]) Collection() *mongo.Collection {
	return r.col
}

// CreateIndexes creates the index for the configured "id" field of the model.
// If no custom "id" field has been configured, no index is created because
// MongoDB automatically creates an index for the "_id" field.
func (r *ModelRepository[Model, ID]) CreateIndexes(ctx context.Context) error {
	if r.key != "_id" {
		_, err := r.col.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: r.key, Value: 1}},
			Options: options.Index().SetUnique(true),
		})
		return err
	}
	return nil
}

// Save saves the given model to the database using the MongoDB "ReplaceOne"
// command with the upsert option set to true.
func (r *ModelRepository[Model, ID]) Save(ctx context.Context, m Model) error {
	var replacement any = m
	if r.customEncoder != nil {
		repl, err := r.customEncoder(m)
		if err != nil {
			return fmt.Errorf("custom encoder: %w", err)
		}
		replacement = repl
	}

	_, err := r.col.ReplaceOne(ctx, bson.D{{Key: r.key, Value: m.ModelID()}}, replacement, options.Replace().SetUpsert(true))
	return err
}

// Fetch fetches the given model from the database. If the model cannot be found,
// an error that unwraps to model.ErrNotFound is returned.
func (r *ModelRepository[Model, ID]) Fetch(ctx context.Context, id ID) (Model, error) {
	res := r.col.FindOne(ctx, bson.D{{Key: r.key, Value: id}})

	var m Model

	if r.factory != nil {
		m = r.factory(id).(Model)
	}

	if err := r.decode(res, &m); err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return m, fmt.Errorf("decode model: %w", err)
		}

		if r.createIfNotFound && r.factory != nil {
			return m, nil
		}

		return m, fmt.Errorf("%w: %v", model.ErrNotFound, err)
	}

	return m, nil
}

func (r *ModelRepository[Model, ID]) decode(res *mongo.SingleResult, m any) error {
	if r.customDecoder != nil {
		return r.customDecoder(res, m)
	}
	return res.Decode(m)
}

// Use fetches the given model from the database, passes the model to the
// provided function and finally saves the model back to the database.
// If the ModelTransactions option is set to true, the operation is done within
// a MongoDB transaction (must be supported by your MongoDB cluster).
func (r *ModelRepository[Model, ID]) Use(ctx context.Context, id ID, fn func(Model) error) error {
	return r.col.Database().Client().UseSession(ctx, func(ctx mongo.SessionContext) error {
		abort := func(txError error) error {
			if r.transactions {
				if err := ctx.AbortTransaction(ctx); err != nil {
					return fmt.Errorf("failed to abort transaction after error %q: %w", txError, err)
				}
			}
			return txError
		}

		if r.transactions {
			if err := ctx.StartTransaction(); err != nil {
				return fmt.Errorf("start transaction: %w", err)
			}
		}

		m, err := r.Fetch(ctx, id)
		if err != nil {
			if err := abort(err); err != nil {
				return err
			}
			return fmt.Errorf("fetch model: %w", err)
		}

		if err := fn(m); err != nil {
			return err
		}

		if err := r.Save(ctx, m); err != nil {
			if err := abort(err); err != nil {
				return err
			}
			return fmt.Errorf("save model: %w", err)
		}

		if r.transactions {
			if err := ctx.CommitTransaction(ctx); err != nil {
				return fmt.Errorf("commit transaction: %w", err)
			}
		}

		return nil
	})
}

// Delete deletes the given model from the database.
func (r *ModelRepository[Model, ID]) Delete(ctx context.Context, m Model) error {
	_, err := r.col.DeleteOne(ctx, bson.D{{Key: r.key, Value: m.ModelID()}})
	return err
}
