package eventstoreui

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresReader struct {
	pool        *pgxpool.Pool
	table       string
	streamIndex string
	decoder     Decoder
}

func newPostgresReader(cfg StoreConfig, decoder Decoder) (*postgresReader, error) {
	table, err := postgresTable(cfg.Table)
	if err != nil {
		return nil, err
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres url: %w", err)
	}
	if cfg.Database != "" {
		poolCfg.ConnConfig.Database = cfg.Database
	}
	poolCfg.MaxConns = 4
	poolCfg.MinConns = 0
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	return &postgresReader{
		pool:        pool,
		table:       table,
		streamIndex: postgresStreamStartsIndex(cfg.Table),
		decoder:     decoder,
	}, nil
}

func postgresTable(name string) (string, error) {
	parts := strings.Split(name, ".")
	if len(parts) > 2 || len(parts) == 0 {
		return "", fmt.Errorf("invalid postgres table %q", name)
	}
	for _, part := range parts {
		if !storeIDPattern.MatchString(part) {
			return "", fmt.Errorf("invalid postgres table %q", name)
		}
	}
	return pgx.Identifier(parts).Sanitize(), nil
}

func postgresStreamStartsIndex(table string) string {
	const (
		prefix = "goes_ui_"
		suffix = "_stream_starts_v1"
		maxLen = 63
	)
	parts := strings.Split(table, ".")
	base := parts[len(parts)-1]
	name := prefix + base + suffix
	if len(name) <= maxLen {
		return name
	}
	sum := sha256.Sum256([]byte(table))
	hash := fmt.Sprintf("%x", sum[:4])
	base = base[:maxLen-len(prefix)-len(suffix)-len(hash)-1]
	return prefix + base + "_" + hash + suffix
}

func postgresStreamStartsIndexLock(table string) int64 {
	sum := sha256.Sum256([]byte("goes:eventstoreui:stream-index:" + table))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

func (reader *postgresReader) Ping(ctx context.Context) error { return reader.pool.Ping(ctx) }
func (reader *postgresReader) Close()                         { reader.pool.Close() }

func (reader *postgresReader) EnsureIndexes(ctx context.Context) error {
	conn, err := reader.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection to ensure required index %q: %w", reader.streamIndex, err)
	}
	lockID := postgresStreamStartsIndexLock(reader.table)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		conn.Release()
		return fmt.Errorf("lock required index %q setup: %w", reader.streamIndex, err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", lockID); err != nil {
			_ = conn.Conn().Close(unlockCtx)
		}
		conn.Release()
	}()

	exists, valid, err := reader.streamIndexStatus(ctx, conn)
	if err != nil {
		return err
	}
	if exists {
		if !valid {
			return fmt.Errorf("required index %q on %s exists but is invalid; drop it and restart the UI", reader.streamIndex, reader.table)
		}
		return nil
	}

	ddl := fmt.Sprintf(`CREATE INDEX CONCURRENTLY IF NOT EXISTS %s ON %s (time)
		INCLUDE (aggregate_name, aggregate_id)
		WHERE aggregate_version = 1 AND aggregate_name IS NOT NULL AND aggregate_id IS NOT NULL`,
		pgx.Identifier{reader.streamIndex}.Sanitize(), reader.table)
	if _, err := conn.Exec(ctx, ddl); err != nil {
		// Another UI instance may have created the same index concurrently.
		if exists, valid, checkErr := reader.streamIndexStatus(ctx, conn); checkErr == nil && exists && valid {
			return nil
		}
		return fmt.Errorf(
			"create required index %q on %s: %w (the PostgreSQL user must own the event table; create it manually with: %s)",
			reader.streamIndex, reader.table, err, ddl,
		)
	}
	return nil
}

type postgresRowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (reader *postgresReader) streamIndexStatus(
	ctx context.Context,
	querier postgresRowQuerier,
) (exists, valid bool, err error) {
	const query = `SELECT i.indisvalid
		FROM pg_catalog.pg_index i
		JOIN pg_catalog.pg_class index_class ON index_class.oid = i.indexrelid
		WHERE i.indrelid = to_regclass($1) AND index_class.relname = $2`
	if err := querier.QueryRow(ctx, query, reader.table, reader.streamIndex).Scan(&valid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("inspect required index %q on %s: %w", reader.streamIndex, reader.table, err)
	}
	return true, valid, nil
}

func (reader *postgresReader) Summary(ctx context.Context) (Summary, error) {
	sql := fmt.Sprintf(`SELECT COUNT(*), COUNT(DISTINCT name), COUNT(DISTINCT aggregate_name), MAX(time)
		FROM %s`, reader.table)
	var summary Summary
	var latest *int64
	if err := reader.pool.QueryRow(ctx, sql).Scan(
		&summary.TotalEvents,
		&summary.EventTypes,
		&summary.AggregateTypes,
		&latest,
	); err != nil {
		return Summary{}, fmt.Errorf("query summary: %w", err)
	}
	if latest != nil {
		t := time.Unix(0, *latest).UTC()
		summary.LatestEventTime = &t
	}
	return summary, nil
}

func (reader *postgresReader) Facets(ctx context.Context) (Facets, error) {
	eventNames, err := reader.postgresFacets(ctx, "name", "name IS NOT NULL")
	if err != nil {
		return Facets{}, err
	}
	aggregateNames, err := reader.postgresFacets(ctx, "aggregate_name", "aggregate_name IS NOT NULL")
	if err != nil {
		return Facets{}, err
	}
	return Facets{EventNames: eventNames, AggregateNames: aggregateNames}, nil
}

func (reader *postgresReader) postgresFacets(ctx context.Context, column, condition string) ([]Facet, error) {
	sql := fmt.Sprintf(`SELECT %s, COUNT(*) FROM %s WHERE %s GROUP BY %s ORDER BY COUNT(*) DESC, %s ASC`,
		column, reader.table, condition, column, column)
	rows, err := reader.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("query %s facets: %w", column, err)
	}
	defer rows.Close()
	facets := make([]Facet, 0)
	for rows.Next() {
		var facet Facet
		if err := rows.Scan(&facet.Value, &facet.Count); err != nil {
			return nil, fmt.Errorf("scan %s facet: %w", column, err)
		}
		facets = append(facets, facet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s facets: %w", column, err)
	}
	return facets, nil
}

func (reader *postgresReader) EventMetadata(ctx context.Context, after time.Time) ([]eventMetadata, error) {
	sql := fmt.Sprintf(`SELECT id, name, aggregate_name, time FROM %s WHERE time >= $1 ORDER BY time ASC`, reader.table)
	rows, err := reader.pool.Query(ctx, sql, after.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("query event metadata: %w", err)
	}
	defer rows.Close()

	events := make([]eventMetadata, 0)
	for rows.Next() {
		var (
			event         eventMetadata
			aggregateName *string
			timestamp     int64
		)
		if err := rows.Scan(&event.ID, &event.Name, &aggregateName, &timestamp); err != nil {
			return nil, fmt.Errorf("scan event metadata: %w", err)
		}
		if aggregateName != nil {
			event.AggregateName = *aggregateName
		}
		event.Time = time.Unix(0, timestamp).UTC()
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read event metadata: %w", err)
	}
	return events, nil
}

func (reader *postgresReader) Events(ctx context.Context, filter EventFilter) (EventPage, error) {
	limit := normalizedLimit(filter.Limit)
	builder := squirrel.Select(
		"id", "name", "time", "aggregate_id", "aggregate_name", "aggregate_version", "data",
	).From(reader.table).PlaceholderFormat(squirrel.Dollar)

	if len(filter.Names) > 0 {
		builder = builder.Where(squirrel.Eq{"name": filter.Names})
	}
	if len(filter.AggregateNames) > 0 {
		builder = builder.Where(squirrel.Eq{"aggregate_name": filter.AggregateNames})
	}
	if filter.AggregateID != "" {
		id, err := uuid.Parse(filter.AggregateID)
		if err != nil {
			return EventPage{}, fmt.Errorf("invalid aggregate id: %w", err)
		}
		builder = builder.Where(squirrel.Eq{"aggregate_id": id})
	}
	if filter.From != nil {
		builder = builder.Where(squirrel.GtOrEq{"time": filter.From.UnixNano()})
	}
	if filter.To != nil {
		builder = builder.Where(squirrel.LtOrEq{"time": filter.To.UnixNano()})
	}
	if filter.Cursor != "" {
		var cursor eventCursor
		if err := decodeCursor(filter.Cursor, &cursor); err != nil {
			return EventPage{}, err
		}
		builder = builder.Where(squirrel.Lt{"time": cursor.TimeNano})
	}

	sql, args, err := builder.OrderBy("time DESC").Limit(uint64(limit + 1)).ToSql()
	if err != nil {
		return EventPage{}, fmt.Errorf("build event query: %w", err)
	}
	rows, err := reader.pool.Query(ctx, sql, args...)
	if err != nil {
		return EventPage{}, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	items := make([]Event, 0, limit+1)
	for rows.Next() {
		event, err := reader.scanEvent(rows)
		if err != nil {
			return EventPage{}, err
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
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

func (reader *postgresReader) Event(ctx context.Context, rawID string) (Event, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return Event{}, fmt.Errorf("invalid event id: %w", err)
	}
	sql := fmt.Sprintf(`SELECT id, name, time, aggregate_id, aggregate_name, aggregate_version, data FROM %s WHERE id = $1`, reader.table)
	event, err := reader.scanEvent(reader.pool.QueryRow(ctx, sql, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("query event: %w", err)
	}
	return event, nil
}

func (reader *postgresReader) StreamStarts(ctx context.Context, after time.Time) ([]streamMetadata, error) {
	builder := squirrel.Select("aggregate_name", "aggregate_id", "time").
		From(reader.table).
		Where(squirrel.Eq{"aggregate_version": 1}).
		Where("aggregate_name IS NOT NULL AND aggregate_id IS NOT NULL").
		PlaceholderFormat(squirrel.Dollar)
	if !after.IsZero() {
		builder = builder.Where(squirrel.GtOrEq{"time": after.UnixNano()})
	}
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build stream start query: %w", err)
	}
	rows, err := reader.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query stream starts: %w", err)
	}
	defer rows.Close()

	streams := make([]streamMetadata, 0)
	for rows.Next() {
		var stream streamMetadata
		var timestamp int64
		if err := rows.Scan(&stream.AggregateName, &stream.AggregateID, &timestamp); err != nil {
			return nil, fmt.Errorf("scan stream start: %w", err)
		}
		stream.CreatedAt = time.Unix(0, timestamp).UTC()
		streams = append(streams, stream)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read stream starts: %w", err)
	}
	return streams, nil
}

func (reader *postgresReader) StreamEvents(
	ctx context.Context,
	aggregateName, rawID string,
	afterVersion, requestedLimit int,
) (EventPage, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return EventPage{}, fmt.Errorf("invalid aggregate id: %w", err)
	}
	limit := normalizedLimit(requestedLimit)
	builder := squirrel.Select(
		"id", "name", "time", "aggregate_id", "aggregate_name", "aggregate_version", "data",
	).From(reader.table).
		Where(squirrel.Eq{"aggregate_name": aggregateName, "aggregate_id": id}).
		PlaceholderFormat(squirrel.Dollar)
	if afterVersion > 0 {
		builder = builder.Where(squirrel.Gt{"aggregate_version": afterVersion})
	}
	sql, args, err := builder.OrderBy("aggregate_version ASC").Limit(uint64(limit + 1)).ToSql()
	if err != nil {
		return EventPage{}, fmt.Errorf("build stream event query: %w", err)
	}
	rows, err := reader.pool.Query(ctx, sql, args...)
	if err != nil {
		return EventPage{}, fmt.Errorf("query stream events: %w", err)
	}
	defer rows.Close()

	items := make([]Event, 0, limit+1)
	for rows.Next() {
		event, err := reader.scanEvent(rows)
		if err != nil {
			return EventPage{}, err
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
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

type postgresScanner interface {
	Scan(...any) error
}

func (reader *postgresReader) scanEvent(scanner postgresScanner) (Event, error) {
	var (
		id               uuid.UUID
		name             string
		timeNano         int64
		aggregateID      *uuid.UUID
		aggregateName    *string
		aggregateVersion *int
		raw              []byte
	)
	if err := scanner.Scan(&id, &name, &timeNano, &aggregateID, &aggregateName, &aggregateVersion, &raw); err != nil {
		return Event{}, err
	}
	event := Event{ID: id.String(), Name: name, Time: time.Unix(0, timeNano).UTC()}
	if aggregateID != nil && aggregateName != nil && aggregateVersion != nil {
		event.Aggregate = &AggregateRef{Name: *aggregateName, ID: aggregateID.String(), Version: *aggregateVersion}
	}
	event.Data, event.DataDecodeError = decodeEventData(reader.decoder, name, raw)
	return event, nil
}
