package eventstoreui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoReader struct {
	client         *mongo.Client
	collection     *mongo.Collection
	eventTimeIndex string
	decoder        Decoder
}

const (
	mongoStreamStartsIndex = "goes_ui_stream_starts_v1"
	mongoEventTimeIndex    = "goes_ui_event_time_v1"
)

type mongoIndexDefinition struct {
	Name                    string `bson:"name"`
	Keys                    bson.D `bson:"key"`
	PartialFilterExpression bson.D `bson:"partialFilterExpression"`
}

type mongoEvent struct {
	ID               uuid.UUID `bson:"id"`
	Name             string    `bson:"name"`
	Time             time.Time `bson:"time"`
	TimeNano         int64     `bson:"timeNano"`
	AggregateName    string    `bson:"aggregateName"`
	AggregateID      uuid.UUID `bson:"aggregateId"`
	AggregateVersion int       `bson:"aggregateVersion"`
	Data             []byte    `bson:"data"`
}

func newMongoReader(cfg StoreConfig, decoder Decoder) (*mongoReader, error) {
	database := cfg.Database
	if database == "" {
		parsed, err := url.Parse(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("parse mongo url: %w", err)
		}
		database = strings.Trim(parsed.Path, "/")
	}
	if database == "" {
		database = "event"
	}
	if strings.Contains(database, "/") {
		return nil, fmt.Errorf("invalid mongo database %q", database)
	}
	if cfg.Collection == "" || strings.ContainsRune(cfg.Collection, '\x00') {
		return nil, fmt.Errorf("invalid mongo collection %q", cfg.Collection)
	}
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.URL))
	if err != nil {
		return nil, fmt.Errorf("create mongo client: %w", err)
	}
	return &mongoReader{
		client: client, collection: client.Database(database).Collection(cfg.Collection), decoder: decoder,
	}, nil
}

func (reader *mongoReader) Ping(ctx context.Context) error {
	return reader.client.Ping(ctx, nil)
}

func (reader *mongoReader) EnsureIndexes(ctx context.Context) error {
	if err := reader.ensureStreamStartsIndex(ctx); err != nil {
		return err
	}
	return reader.ensureEventTimeIndex(ctx)
}

func (reader *mongoReader) ensureStreamStartsIndex(ctx context.Context) error {
	found, err := reader.hasStreamStartsIndex(ctx)
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	_, err = reader.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "timeNano", Value: 1}},
		Options: options.Index().
			SetName(mongoStreamStartsIndex).
			SetPartialFilterExpression(bson.D{{Key: "aggregateVersion", Value: 1}}),
	})
	if err == nil {
		return nil
	}

	// Another UI instance may have created the same index concurrently.
	if found, checkErr := reader.hasStreamStartsIndex(ctx); checkErr == nil && found {
		return nil
	}
	return fmt.Errorf(
		"create required index %q on collection %q: %w (the MongoDB user needs createIndex permission)",
		mongoStreamStartsIndex, reader.collection.Name(), err,
	)
}

func (reader *mongoReader) ensureEventTimeIndex(ctx context.Context) error {
	name, found, err := reader.findEventTimeIndex(ctx)
	if err != nil {
		return err
	}
	if found {
		reader.eventTimeIndex = name
		return nil
	}

	_, err = reader.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "timeNano", Value: 1}},
		Options: options.Index().SetName(mongoEventTimeIndex),
	})
	if err == nil {
		reader.eventTimeIndex = mongoEventTimeIndex
		return nil
	}
	if name, found, checkErr := reader.findEventTimeIndex(ctx); checkErr == nil && found {
		reader.eventTimeIndex = name
		return nil
	}
	return fmt.Errorf(
		"create required event-time index %q on collection %q: %w (the MongoDB user needs createIndex permission)",
		mongoEventTimeIndex, reader.collection.Name(), err,
	)
}

func (reader *mongoReader) findEventTimeIndex(ctx context.Context) (string, bool, error) {
	cursor, err := reader.collection.Indexes().List(ctx)
	if err != nil {
		return "", false, fmt.Errorf("list indexes on collection %q: %w", reader.collection.Name(), err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var index mongoIndexDefinition
		if err := cursor.Decode(&index); err != nil {
			return "", false, fmt.Errorf("decode index on collection %q: %w", reader.collection.Name(), err)
		}
		if isMongoEventTimeIndex(index) {
			return index.Name, true, nil
		}
	}
	if err := cursor.Err(); err != nil {
		return "", false, fmt.Errorf("read indexes on collection %q: %w", reader.collection.Name(), err)
	}
	return "", false, nil
}

func (reader *mongoReader) hasStreamStartsIndex(ctx context.Context) (bool, error) {
	cursor, err := reader.collection.Indexes().List(ctx)
	if err != nil {
		var commandErr mongo.CommandError
		if errors.As(err, &commandErr) && commandErr.HasErrorCode(26) {
			return false, nil
		}
		return false, fmt.Errorf("list indexes on collection %q: %w", reader.collection.Name(), err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var index mongoIndexDefinition
		if err := cursor.Decode(&index); err != nil {
			return false, fmt.Errorf("decode index on collection %q: %w", reader.collection.Name(), err)
		}
		if index.Name != mongoStreamStartsIndex {
			continue
		}
		if !isMongoStreamStartsIndex(index) {
			return false, fmt.Errorf(
				"required index %q on collection %q has an incompatible definition",
				mongoStreamStartsIndex, reader.collection.Name(),
			)
		}
		return true, nil
	}
	if err := cursor.Err(); err != nil {
		return false, fmt.Errorf("read indexes on collection %q: %w", reader.collection.Name(), err)
	}
	return false, nil
}

func isMongoStreamStartsIndex(index mongoIndexDefinition) bool {
	return len(index.Keys) == 1 &&
		index.Keys[0].Key == "timeNano" && numericOne(index.Keys[0].Value) &&
		len(index.PartialFilterExpression) == 1 &&
		index.PartialFilterExpression[0].Key == "aggregateVersion" &&
		numericOne(index.PartialFilterExpression[0].Value)
}

func isMongoEventTimeIndex(index mongoIndexDefinition) bool {
	return len(index.Keys) == 1 &&
		index.Keys[0].Key == "timeNano" && numericDirection(index.Keys[0].Value) &&
		len(index.PartialFilterExpression) == 0
}

func numericOne(value any) bool {
	switch value := value.(type) {
	case int:
		return value == 1
	case int32:
		return value == 1
	case int64:
		return value == 1
	case float64:
		return value == 1
	default:
		return false
	}
}

func numericDirection(value any) bool {
	switch value := value.(type) {
	case int:
		return value == 1 || value == -1
	case int32:
		return value == 1 || value == -1
	case int64:
		return value == 1 || value == -1
	case float64:
		return value == 1 || value == -1
	default:
		return false
	}
}

func (reader *mongoReader) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = reader.client.Disconnect(ctx)
}

func (reader *mongoReader) Summary(ctx context.Context) (Summary, error) {
	var summary Summary
	var err error
	if summary.TotalEvents, err = reader.collection.CountDocuments(ctx, bson.D{}); err != nil {
		return Summary{}, fmt.Errorf("count events: %w", err)
	}
	var eventNames []string
	if err := reader.collection.Distinct(ctx, "name", bson.D{{Key: "name", Value: bson.D{{Key: "$ne", Value: ""}}}}).Decode(&eventNames); err != nil {
		return Summary{}, fmt.Errorf("count event types: %w", err)
	}
	summary.EventTypes = int64(len(eventNames))
	var aggregateNames []string
	if err := reader.collection.Distinct(ctx, "aggregateName", bson.D{{Key: "aggregateName", Value: bson.D{{Key: "$ne", Value: ""}}}}).Decode(&aggregateNames); err != nil {
		return Summary{}, fmt.Errorf("count aggregate types: %w", err)
	}
	summary.AggregateTypes = int64(len(aggregateNames))

	var latest struct {
		TimeNano int64 `bson:"timeNano"`
	}
	err = reader.collection.FindOne(ctx, bson.D{}, options.FindOne().
		SetSort(bson.D{{Key: "timeNano", Value: -1}}).
		SetProjection(bson.D{{Key: "timeNano", Value: 1}})).Decode(&latest)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return Summary{}, fmt.Errorf("query latest event: %w", err)
	}
	if err == nil {
		t := time.Unix(0, latest.TimeNano).UTC()
		summary.LatestEventTime = &t
	}
	return summary, nil
}

func (reader *mongoReader) Facets(ctx context.Context) (Facets, error) {
	eventNames, err := reader.mongoFacets(ctx, "name", bson.D{{Key: "name", Value: bson.D{{Key: "$ne", Value: ""}}}})
	if err != nil {
		return Facets{}, err
	}
	aggregateNames, err := reader.mongoFacets(ctx, "aggregateName", bson.D{{Key: "aggregateName", Value: bson.D{{Key: "$ne", Value: ""}}}})
	if err != nil {
		return Facets{}, err
	}
	return Facets{EventNames: eventNames, AggregateNames: aggregateNames}, nil
}

func (reader *mongoReader) mongoFacets(ctx context.Context, field string, filter bson.D) ([]Facet, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$" + field}, {Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}, {Key: "_id", Value: 1}}}},
	}
	cursor, err := reader.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("query %s facets: %w", field, err)
	}
	defer cursor.Close(ctx)
	var rows []struct {
		Value string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read %s facets: %w", field, err)
	}
	facets := make([]Facet, len(rows))
	for i, row := range rows {
		facets[i] = Facet{Value: row.Value, Count: row.Count}
	}
	return facets, nil
}

func (reader *mongoReader) EventMetadata(ctx context.Context, after time.Time) ([]eventMetadata, error) {
	filter := bson.D{{Key: "timeNano", Value: bson.D{{Key: "$gte", Value: after.UnixNano()}}}}
	findOptions := options.Find().
		SetProjection(bson.D{
			{Key: "_id", Value: 0},
			{Key: "id", Value: 1},
			{Key: "name", Value: 1},
			{Key: "aggregateName", Value: 1},
			{Key: "time", Value: 1},
			{Key: "timeNano", Value: 1},
		}).
		SetSort(bson.D{{Key: "timeNano", Value: 1}}).
		SetBatchSize(2_000)
	if reader.eventTimeIndex != "" {
		findOptions.SetHint(reader.eventTimeIndex)
	}
	cursor, err := reader.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("query event metadata: %w", err)
	}
	defer cursor.Close(ctx)

	events := make([]eventMetadata, 0)
	for cursor.Next(ctx) {
		var row struct {
			ID            uuid.UUID `bson:"id"`
			Name          string    `bson:"name"`
			AggregateName string    `bson:"aggregateName"`
			Time          time.Time `bson:"time"`
			TimeNano      int64     `bson:"timeNano"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode event metadata: %w", err)
		}
		timestamp := row.TimeNano
		if timestamp == 0 && !row.Time.IsZero() {
			timestamp = row.Time.UnixNano()
		}
		events = append(events, eventMetadata{
			ID: row.ID, Name: row.Name, AggregateName: row.AggregateName, Time: time.Unix(0, timestamp).UTC(),
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("read event metadata: %w", err)
	}
	return events, nil
}

func (reader *mongoReader) Events(ctx context.Context, filter EventFilter) (EventPage, error) {
	limit := normalizedLimit(filter.Limit)
	conditions := make(bson.A, 0, 8)
	if len(filter.Names) > 0 {
		conditions = append(conditions, bson.D{{Key: "name", Value: bson.D{{Key: "$in", Value: filter.Names}}}})
	}
	if len(filter.AggregateNames) > 0 {
		conditions = append(conditions, bson.D{{Key: "aggregateName", Value: bson.D{{Key: "$in", Value: filter.AggregateNames}}}})
	}
	if filter.AggregateID != "" {
		id, err := uuid.Parse(filter.AggregateID)
		if err != nil {
			return EventPage{}, fmt.Errorf("invalid aggregate id: %w", err)
		}
		conditions = append(conditions, bson.D{{Key: "aggregateId", Value: id}})
	}
	if filter.From != nil || filter.To != nil {
		rangeFilter := bson.D{}
		if filter.From != nil {
			rangeFilter = append(rangeFilter, bson.E{Key: "$gte", Value: filter.From.UnixNano()})
		}
		if filter.To != nil {
			rangeFilter = append(rangeFilter, bson.E{Key: "$lte", Value: filter.To.UnixNano()})
		}
		conditions = append(conditions, bson.D{{Key: "timeNano", Value: rangeFilter}})
	}
	if filter.Cursor != "" {
		var cursor eventCursor
		if err := decodeCursor(filter.Cursor, &cursor); err != nil {
			return EventPage{}, err
		}
		conditions = append(conditions, bson.D{{Key: "timeNano", Value: bson.D{{Key: "$lt", Value: cursor.TimeNano}}}})
	}
	query := andMongoConditions(conditions)
	cursor, err := reader.collection.Find(ctx, query, options.Find().
		SetSort(bson.D{{Key: "timeNano", Value: -1}}).
		SetLimit(int64(limit+1)))
	if err != nil {
		return EventPage{}, fmt.Errorf("query events: %w", err)
	}
	defer cursor.Close(ctx)
	items := make([]Event, 0, limit+1)
	for cursor.Next(ctx) {
		var raw mongoEvent
		if err := cursor.Decode(&raw); err != nil {
			return EventPage{}, fmt.Errorf("decode event document: %w", err)
		}
		items = append(items, reader.event(raw))
	}
	if err := cursor.Err(); err != nil {
		return EventPage{}, fmt.Errorf("read events: %w", err)
	}
	page := EventPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		page.Items = items[:limit]
		page.NextCursor = encodeCursor(eventCursor{TimeNano: last.Time.UnixNano()})
	}
	return page, nil
}

func (reader *mongoReader) Event(ctx context.Context, rawID string) (Event, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return Event{}, fmt.Errorf("invalid event id: %w", err)
	}
	var raw mongoEvent
	if err := reader.collection.FindOne(ctx, bson.D{{Key: "id", Value: id}}).Decode(&raw); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Event{}, ErrNotFound
		}
		return Event{}, fmt.Errorf("query event: %w", err)
	}
	return reader.event(raw), nil
}

func (reader *mongoReader) StreamStarts(ctx context.Context, after time.Time) ([]streamMetadata, error) {
	filter := bson.D{{Key: "aggregateVersion", Value: 1}}
	if !after.IsZero() {
		filter = append(filter, bson.E{Key: "timeNano", Value: bson.D{{Key: "$gte", Value: after.UnixNano()}}})
	}
	cursor, err := reader.collection.Find(ctx, filter, options.Find().
		SetProjection(bson.D{
			{Key: "_id", Value: 0},
			{Key: "aggregateName", Value: 1},
			{Key: "aggregateId", Value: 1},
			{Key: "time", Value: 1},
			{Key: "timeNano", Value: 1},
		}).
		SetHint(mongoStreamStartsIndex).
		SetBatchSize(2_000))
	if err != nil {
		return nil, fmt.Errorf("query stream starts: %w", err)
	}
	defer cursor.Close(ctx)

	streams := make([]streamMetadata, 0)
	for cursor.Next(ctx) {
		var row struct {
			AggregateName string    `bson:"aggregateName"`
			AggregateID   uuid.UUID `bson:"aggregateId"`
			Time          time.Time `bson:"time"`
			TimeNano      int64     `bson:"timeNano"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode stream start: %w", err)
		}
		timestamp := row.TimeNano
		if timestamp == 0 && !row.Time.IsZero() {
			timestamp = row.Time.UnixNano()
		}
		streams = append(streams, streamMetadata{
			AggregateName: row.AggregateName,
			AggregateID:   row.AggregateID,
			CreatedAt:     time.Unix(0, timestamp).UTC(),
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("read stream starts: %w", err)
	}
	return streams, nil
}

func (reader *mongoReader) StreamEvents(
	ctx context.Context,
	aggregateName, rawID string,
	afterVersion, requestedLimit int,
) (EventPage, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return EventPage{}, fmt.Errorf("invalid aggregate id: %w", err)
	}
	limit := normalizedLimit(requestedLimit)
	filter := bson.D{{Key: "aggregateName", Value: aggregateName}, {Key: "aggregateId", Value: id}}
	if afterVersion > 0 {
		filter = append(filter, bson.E{Key: "aggregateVersion", Value: bson.D{{Key: "$gt", Value: afterVersion}}})
	}
	cursor, err := reader.collection.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "aggregateVersion", Value: 1}}).
		SetLimit(int64(limit+1)))
	if err != nil {
		return EventPage{}, fmt.Errorf("query stream events: %w", err)
	}
	defer cursor.Close(ctx)

	items := make([]Event, 0, limit+1)
	for cursor.Next(ctx) {
		var raw mongoEvent
		if err := cursor.Decode(&raw); err != nil {
			return EventPage{}, fmt.Errorf("decode event document: %w", err)
		}
		items = append(items, reader.event(raw))
	}
	if err := cursor.Err(); err != nil {
		return EventPage{}, fmt.Errorf("read stream events: %w", err)
	}
	page := EventPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		page.Items = items[:limit]
		page.NextCursor = encodeCursor(versionCursor{Version: last.Aggregate.Version})
	}
	return page, nil
}

func (reader *mongoReader) event(raw mongoEvent) Event {
	timestamp := raw.TimeNano
	if timestamp == 0 && !raw.Time.IsZero() {
		timestamp = raw.Time.UnixNano()
	}
	event := Event{ID: raw.ID.String(), Name: raw.Name, Time: time.Unix(0, timestamp).UTC()}
	if raw.AggregateName != "" && raw.AggregateID != uuid.Nil {
		event.Aggregate = &AggregateRef{Name: raw.AggregateName, ID: raw.AggregateID.String(), Version: raw.AggregateVersion}
	}
	event.Data, event.DataDecodeError = decodeEventData(reader.decoder, raw.Name, raw.Data)
	return event
}

func andMongoConditions(conditions bson.A) bson.D {
	if len(conditions) == 0 {
		return bson.D{}
	}
	if len(conditions) == 1 {
		return conditions[0].(bson.D)
	}
	return bson.D{{Key: "$and", Value: conditions}}
}
