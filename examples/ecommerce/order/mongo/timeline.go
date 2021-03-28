package mongo

import (
	"context"
	"ecommerce/order"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type timelineRepository struct {
	col *mongo.Collection
}

func TimelineRepository(ctx context.Context, db *mongo.Database) (order.TimelineRepository, error) {
	col := db.Collection("timelines")
	if _, err := col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "id", Value: 1}},
	}); err != nil {
		return nil, err
	}
	return &timelineRepository{col: col}, nil
}

func (r *timelineRepository) Save(ctx context.Context, tl *order.Timeline) error {
	if _, err := r.col.ReplaceOne(ctx, bson.M{"id": tl.ID}, tl, options.Replace().SetUpsert(true)); err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	return nil
}

func (r *timelineRepository) Fetch(ctx context.Context, id uuid.UUID) (*order.Timeline, error) {
	res := r.col.FindOne(ctx, bson.M{"id": id})

	tl := order.NewTimeline(id)
	if err := res.Decode(&tl); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, order.ErrTimelineNotFound
		}
		return nil, fmt.Errorf("mongo: %w", err)
	}

	return tl, nil
}
