package indices

import (
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EventStore provides the builtin index models for the MongoDB event store.
var EventStore = EventStoreIndices{
	ID: mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetName("goes_id").SetUnique(true),
	},

	Name: mongo.IndexModel{Keys: bson.D{{Key: "name", Value: 1}}},

	NameAndTime: mongo.IndexModel{
		Keys: bson.D{
			{Key: "name", Value: 1},
			{Key: "timeNano", Value: 1},
		},
		Options: options.Index().SetName("goes_name_time"),
	},

	AggregateNameAndVersion: mongo.IndexModel{
		Keys: bson.D{
			{Key: "aggregateName", Value: 1},
			{Key: "aggregateVersion", Value: 1},
		},
		Options: options.Index().SetName("goes_aname_aversion"),
	},

	AggregateNameAndIDAndVersion: mongo.IndexModel{
		Keys: bson.D{
			{Key: "aggregateName", Value: 1},
			{Key: "aggregateId", Value: 1},
			{Key: "aggregateVersion", Value: 1},
		},
		Options: options.Index().SetName("goes_aname_aid_aversion").
			SetUnique(true).
			SetPartialFilterExpression(bson.D{
				{Key: "aggregateName", Value: bson.D{{Key: "$exists", Value: true}, {Key: "$gt", Value: ""}}},
				{Key: "aggregateId", Value: bson.D{{Key: "$exists", Value: true}, {Key: "$gt", Value: uuid.UUID{}}}},
				{Key: "aggregateVersion", Value: bson.D{{Key: "$exists", Value: true}, {Key: "$gt", Value: 0}}},
			}),
	},

	AggregateID: mongo.IndexModel{
		Keys:    bson.D{{Key: "aggregateId", Value: 1}},
		Options: options.Index().SetName("goes_aid"),
	},

	AggregateVersion: mongo.IndexModel{
		Keys:    bson.D{{Key: "aggregateVersion", Value: 1}},
		Options: options.Index().SetName("goes_aversion"),
	},

	NameAndVersion: mongo.IndexModel{
		Keys: bson.D{
			{Key: "name", Value: 1},
			{Key: "aggregateVersion", Value: 1},
		},
		Options: options.Index().SetName("goes_name_aversion"),
	},
}

// EventStoreIndices provides the builtin index models for the MongoDB event store.
type EventStoreIndices struct {
	// Core indices

	// ID creates an index for the event id.
	ID mongo.IndexModel

	// Name creates an index for the event name.
	Name mongo.IndexModel

	// NameAndTime creates a compound index for the event name and id.
	NameAndTime mongo.IndexModel

	// AggregateNameAndVersion creates a compound index for the aggregate name and version.
	AggregateNameAndVersion mongo.IndexModel

	// AggregateNameAndIDAndVersion creates a compound index for the aggregate name, id, and version.
	AggregateNameAndIDAndVersion mongo.IndexModel

	// Edge-case indices

	// ISOTime creates an index for the ISO time field. Usually, this is not
	// needed because an index is already created for the nano time field.
	ISOTime mongo.IndexModel

	// AggregateID creates an index for the aggregate id.
	AggregateID mongo.IndexModel

	// AggregateVersion creates an index for the aggregate version.
	AggregateVersion mongo.IndexModel

	// NameAndVersion creates a compound index for the event name and version.
	NameAndVersion mongo.IndexModel
}

// EventStoreCore returns the core index models for the MongoDB event store.
func EventStoreCore() []mongo.IndexModel {
	return []mongo.IndexModel{
		EventStore.ID,
		EventStore.Name,
		EventStore.NameAndTime,
		EventStore.AggregateNameAndVersion,
		EventStore.AggregateNameAndIDAndVersion,
	}
}

// EventStoreEdge returns all edge-case index models for the MongoDB event store.
func EventStoreEdge() []mongo.IndexModel {
	return []mongo.IndexModel{
		EventStore.ISOTime,
		EventStore.AggregateID,
		EventStore.AggregateVersion,
		EventStore.NameAndVersion,
	}
}
