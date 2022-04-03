package indices

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// IndexFunc creates indexes.
type IndexFunc func(context.Context, *mongo.IndexView) error
