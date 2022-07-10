package postgres

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
	"github.com/modernice/goes/internal/slice"
)

var _ event.Store = &EventStore{}

// EventStore is a PostgreSQL event store.
type EventStore struct {
	onceConnect   sync.Once
	connectionURL string
	database      string
	table         string
	pool          *pgxpool.Pool
	enc           codec.Encoding
}

// EventStoreOption is an optionn for the PostgreSQL event store.
type EventStoreOption func(*EventStore)

// URL returns an EventStoreOption that specifies the connection string to the
// PostgreSQL server.
func URL(url string) EventStoreOption {
	return func(store *EventStore) {
		store.connectionURL = url
	}
}

// Database returns an EventStoreOption that configures the used database.
// Defaults to "goes".
func Database(name string) EventStoreOption {
	if name = strings.TrimSpace(name); name == "" {
		panic("database name cannot be empty")
	}

	return func(store *EventStore) {
		store.database = name
	}
}

// Table returns an EventStoreOption that configures the used table for events.
// Defaults to "events".
func Table(name string) EventStoreOption {
	if name = strings.TrimSpace(name); name == "" {
		panic(fmt.Errorf("table name cannot be empty"))
	}

	return func(store *EventStore) {
		store.table = name
	}
}

// NewEventStore returns a new PostgreSQL event store. If not otherwise
// specified using the URL() option, os.Getenv("POSTGRES_EVENTSTORE") is used as
// the connection string.
func NewEventStore(enc codec.Encoding, opts ...EventStoreOption) *EventStore {
	store := &EventStore{
		enc:           enc,
		database:      "goes",
		table:         "events",
		connectionURL: os.Getenv("POSTGRES_EVENTSTORE"),
	}
	for _, opt := range opts {
		opt(store)
	}
	return store
}

// Pool returns the underlying Postgres connection pool. Pool must only be
// called AFTER the connection to Postgres has been established. Otherwise the
// returned Pool will be nil. It is not thread-safe to call Pool concurrently
// with Connect.
func (store *EventStore) Pool() *pgxpool.Pool {
	return store.pool
}

// Connect connects to the PostgreSQL server. Connect is automatically called
// from the Insert, Find, Query, and Delete methods if not called explicitly.
func (store *EventStore) Connect(ctx context.Context) error {
	return store.connectOnce(ctx)
}

func (store *EventStore) connectOnce(ctx context.Context) error {
	var err error
	store.onceConnect.Do(func() {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		if err = store.connect(ctx); err != nil {
			return
		}

		if err = store.createDatabase(ctx); err != nil {
			return
		}

		if err = store.useDatabase(ctx); err != nil {
			return
		}

		if err = store.createTable(ctx); err != nil {
			return
		}

		if err = store.createIndexes(ctx); err != nil {
			return
		}
	})
	return err
}

func (store *EventStore) connect(ctx context.Context) error {
	url := store.connectionURL
	if url == "" {
		return fmt.Errorf("missing connection string")
	}

	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("parse connection string: %w", err)
	}

	pool, err := pgxpool.Connect(ctx, cfg.ConnString())
	if err != nil {
		return fmt.Errorf("connect to postgres: %w [url=%s]", err, url)
	}
	store.pool = pool

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	return nil
}

func (store *EventStore) createDatabase(ctx context.Context) error {
	var exists bool
	err := store.pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1)", store.database).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check if %q database exists: %w", store.database, err)
	}

	if exists {
		return nil
	}

	if _, err := store.pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", store.database)); err != nil {
		return fmt.Errorf("create %q database: %w", "goes", err)
	}

	return nil
}

func (store *EventStore) useDatabase(ctx context.Context) error {
	store.pool.Close()

	cfg, err := pgx.ParseConfig(store.connectionURL)
	if err != nil {
		return fmt.Errorf("parse connection string: %w [url=%s]", err, store.connectionURL)
	}

	purl, err := url.Parse(cfg.ConnString())
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	purl.Path = "/" + store.database
	connURL := purl.String()

	pool, err := pgxpool.Connect(ctx, connURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w [url=%s]", err, cfg.ConnString())
	}
	store.pool = pool

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	return nil
}

func (store *EventStore) createTable(ctx context.Context) error {
	if _, err := store.pool.Exec(ctx, eventTableSQL(store.table)); err != nil {
		return fmt.Errorf("create %q table: %w", store.table, err)
	}
	return nil
}

func (store *EventStore) createIndexes(ctx context.Context) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	indexes := []struct {
		name   string
		fields []string
		unique bool
	}{
		{
			name:   "goes_name",
			fields: []string{"name"},
		},
		{
			name:   "goes_time",
			fields: []string{"time"},
		},
		{
			name:   "goes_aggregate",
			fields: []string{"aggregate_id", "aggregate_name", "aggregate_version"},
			unique: true,
		},
	}

	for _, idx := range indexes {
		schema := indexSQL(idx.name, store.table, idx.fields, idx.unique)
		if _, err := tx.Exec(ctx, schema); err != nil {
			return fmt.Errorf("create %q index: %w [fields=%v]", idx.name, err, idx.fields)
		}
	}

	return tx.Commit(ctx)
}

// Insert inserts events into the event store.
func (store *EventStore) Insert(ctx context.Context, events ...event.Event) error {
	if err := store.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, evt := range events {
		aggregateID, aggregateName, aggregateVersion := evt.Aggregate()

		b, err := store.enc.Marshal(evt.Data())
		if err != nil {
			return fmt.Errorf("marshal %q event data: %w", evt.Name(), err)
		}

		aggregateIDVal := aggregateID.String()
		aggregateVersionVal := strconv.Itoa(aggregateVersion)

		if aggregateID == uuid.Nil {
			aggregateIDVal = "NULL"
		} else {
			aggregateIDVal = singleQuote(aggregateIDVal)
		}

		if aggregateName == "" {
			aggregateName = "NULL"
		} else {
			aggregateName = singleQuote(aggregateName)
		}

		if aggregateVersion == 0 {
			aggregateVersionVal = "NULL"
		}

		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO events (
			id, name, time, aggregate_id, aggregate_name, aggregate_version, data
		) VALUES (
			$1, $2, $3, %s, %s, %s, $4
		)`, aggregateIDVal, aggregateName, aggregateVersionVal), evt.ID(), evt.Name(), evt.Time().UnixNano(), b); err != nil {
			return fmt.Errorf("insert %q event: %w", evt.Name(), err)
		}
	}

	return tx.Commit(ctx)
}

// Find fetches the event with the given id from the event store.
func (store *EventStore) Find(ctx context.Context, id uuid.UUID) (event.Event, error) {
	if err := store.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	var evt dbevent
	if err := store.pool.QueryRow(
		ctx,
		`SELECT id, name, time, aggregate_id, aggregate_name, aggregate_version, data FROM events WHERE id = $1`,
		id,
	).Scan(
		&evt.ID,
		&evt.Name,
		&evt.Time,
		&evt.AggregateID,
		&evt.AggregateName,
		&evt.AggregateVersion,
		&evt.Data,
	); err != nil {
		return nil, fmt.Errorf("query event: %w", err)
	}

	return store.decodeEvent(evt)
}

func (store *EventStore) decodeEvent(devt dbevent) (event.Event, error) {
	opts := []event.Option{event.ID(devt.ID), event.Time(time.Unix(0, devt.Time))}
	if devt.AggregateID != nil && devt.AggregateName != nil && devt.AggregateVersion != nil {
		opts = append(opts, event.Aggregate(
			*devt.AggregateID,
			*devt.AggregateName,
			*devt.AggregateVersion,
		))
	}

	data, err := store.enc.Unmarshal(devt.Data, devt.Name)
	if err != nil {
		return nil, fmt.Errorf("unmarshal event data: %w", err)
	}

	return event.New(devt.Name, data, opts...), nil
}

// Query queries the event store for events.
func (store *EventStore) Query(ctx context.Context, query event.Query) (<-chan event.Event, <-chan error, error) {
	sql, args, err := store.buildQuery(query)
	if err != nil {
		return nil, nil, fmt.Errorf("build query: %w", err)
	}

	res, err := store.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query events: %w", err)
	}

	out := make(chan event.Event)
	errs := make(chan error)

	go func() {
		defer close(out)
		defer close(errs)
		defer res.Close()

		for res.Next() {
			var devt dbevent
			if err := res.Scan(&devt.ID, &devt.Name, &devt.Time, &devt.AggregateID, &devt.AggregateName, &devt.AggregateVersion, &devt.Data); err != nil {
				select {
				case <-ctx.Done():
				case errs <- fmt.Errorf("scan row: %w", err):
				}
				return
			}

			evt, err := store.decodeEvent(devt)
			if err != nil {
				select {
				case <-ctx.Done():
				case errs <- fmt.Errorf("decode event: %w", err):
				}
				return
			}

			select {
			case <-ctx.Done():
			case out <- evt:
			}
		}

		if err := res.Err(); err != nil {
			select {
			case <-ctx.Done():
				return
			case errs <- err:
			}
		}
	}()

	return out, errs, nil
}

func (store *EventStore) buildQuery(query event.Query) (string, []any, error) {
	builder := squirrel.
		Select("id", "name", "time", "aggregate_id", "aggregate_name", "aggregate_version", "data").
		From(store.table).
		PlaceholderFormat(squirrel.Dollar)

	if ids := query.AggregateIDs(); len(ids) > 0 {
		builder = builder.Where(buildOREq("aggregate_id", ids))
	}

	if names := query.AggregateNames(); len(names) > 0 {
		builder = builder.Where(squirrel.Eq{"aggregate_name": names})
	}

	if versions := query.AggregateVersions(); versions != nil {
		if exact := versions.Exact(); len(exact) > 0 {
			builder = builder.Where(squirrel.Eq{"aggregate_version": exact})
		}

		if min := versions.Min(); len(min) > 0 {
			builder = builder.Where(buildORGte("aggregate_version", min))
		}

		if max := versions.Max(); len(max) > 0 {
			builder = builder.Where(buildORLte("aggregate_version", max))
		}

		if ranges := versions.Ranges(); len(ranges) > 0 {
			or := make(squirrel.Or, len(ranges))
			for i, r := range ranges {
				or[i] = squirrel.And{
					squirrel.GtOrEq{"aggregate_version": r.Start()},
					squirrel.LtOrEq{"aggregate_version": r.End()},
				}
			}
			builder = builder.Where(or)
		}
	}

	if refs := query.Aggregates(); len(refs) > 0 {
		or := make(squirrel.Or, len(refs))
		for i, ref := range refs {
			and := squirrel.And{squirrel.Eq{"aggregate_name": ref.Name}}

			if ref.ID != uuid.Nil {
				and = append(and, squirrel.Eq{"aggregate_id": ref.ID})
			}

			or[i] = and
		}
		builder = builder.Where(or)
	}

	if ids := query.IDs(); len(ids) > 0 {
		builder = builder.Where(buildOREq("id", ids))
	}

	if names := query.Names(); len(names) > 0 {
		builder = builder.Where(squirrel.Eq{"name": names})
	}

	if times := query.Times(); times != nil {
		if exact := times.Exact(); len(exact) > 0 {
			builder = builder.Where(buildOREq("time", slice.Map(exact, func(t time.Time) int64 {
				return t.UnixNano()
			})))
		}

		if min := times.Min(); !min.IsZero() {
			builder = builder.Where(squirrel.GtOrEq{"time": min.UnixNano()})
		}

		if max := times.Max(); !max.IsZero() {
			builder = builder.Where(squirrel.LtOrEq{"time": max.UnixNano()})
		}

		if ranges := times.Ranges(); len(ranges) > 0 {
			or := make(squirrel.Or, len(ranges))
			for i, r := range ranges {
				or[i] = squirrel.And{
					squirrel.GtOrEq{"time": r.Start().UnixNano()},
					squirrel.LtOrEq{"time": r.End().UnixNano()},
				}
			}
			builder = builder.Where(or)
		}
	}

	if sortings := query.Sortings(); len(sortings) > 0 {
		orders := make([]string, len(sortings))
		for i, sorting := range sortings {
			dir := "ASC"
			if sorting.Dir == event.SortDesc {
				dir = "DESC"
			}

			var field string
			switch sorting.Sort {
			case event.SortAggregateID:
				field = "aggregate_id"
			case event.SortAggregateName:
				field = "aggregate_name"
			case event.SortAggregateVersion:
				field = "aggregate_version"
			case event.SortTime:
				field = "time"
			}

			orders[i] = fmt.Sprintf("%s %s", field, dir)
		}

		builder = builder.OrderBy(orders...)
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return "", nil, fmt.Errorf("build sql: %w", err)
	}

	return sql, args, nil
}

// Delete deletes the given events from the event store.
func (store *EventStore) Delete(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, evt := range events {
		sql, args, err := squirrel.Delete(store.table).Where(squirrel.Eq{"id": evt.ID()}).PlaceholderFormat(squirrel.Dollar).ToSql()
		if err != nil {
			return fmt.Errorf("delete event: %w [id=%s]", err, evt.ID())
		}

		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return fmt.Errorf("delete event: %w [id=%s]", err, evt.ID())
		}
	}

	return tx.Commit(ctx)
}

type dbevent struct {
	ID               uuid.UUID
	Name             string
	Time             int64
	AggregateID      *uuid.UUID
	AggregateName    *string
	AggregateVersion *int
	Data             []byte
}

func buildOREq[S ~[]E, E any](field string, values S) squirrel.Or {
	or := make(squirrel.Or, len(values))
	for i, v := range values {
		or[i] = squirrel.Eq{field: v}
	}
	return or
}

func buildORGte[S ~[]E, E any](field string, values S) squirrel.Or {
	or := make(squirrel.Or, len(values))
	for i, v := range values {
		or[i] = squirrel.GtOrEq{field: v}
	}
	return or
}

func buildORLte[S ~[]E, E any](field string, values S) squirrel.Or {
	or := make(squirrel.Or, len(values))
	for i, v := range values {
		or[i] = squirrel.LtOrEq{field: v}
	}
	return or
}

func eventTableSQL(name string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id UUID PRIMARY KEY NOT NULL,
		name VARCHAR(255) NOT NULL,
		time BIGINT NOT NULL,
		aggregate_id UUID,
		aggregate_name VARCHAR(255),
		aggregate_version INTEGER,
		data JSONB
	)`, name)
}

func indexSQL(name, table string, fields []string, unique bool) string {
	var uniqueOpt string
	if unique {
		uniqueOpt = "UNIQUE"
	}
	return fmt.Sprintf("CREATE %s INDEX IF NOT EXISTS %s ON %s (%s)", uniqueOpt, name, table, strings.Join(fields, ", "))
}

func singleQuote(s string) string {
	return "'" + s + "'"
}
