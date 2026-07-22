package eventstoreui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

// Decoder turns the persisted bytes of a named event into a JSON-serializable
// Go value. codec.Encoding implementations satisfy this interface directly.
type Decoder interface {
	Unmarshal([]byte, string) (any, error)
}

type JSONDecoder struct{}

func (JSONDecoder) Unmarshal(raw []byte, _ string) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var data any
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("event payload contains trailing JSON data")
		}
		return nil, err
	}
	return data, nil
}

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

type StoreInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type AggregateRef struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type Event struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Time            time.Time     `json:"time"`
	Aggregate       *AggregateRef `json:"aggregate,omitempty"`
	Data            any           `json:"data"`
	DataDecodeError string        `json:"dataDecodeError,omitempty"`
}

type Summary struct {
	TotalEvents        int64      `json:"totalEvents"`
	EventTypes         int64      `json:"eventTypes"`
	AggregateTypes     int64      `json:"aggregateTypes"`
	AggregateStreams   int64      `json:"aggregateStreams"`
	LatestEventTime    *time.Time `json:"latestEventTime,omitempty"`
	PayloadDecodeError int64      `json:"payloadDecodeErrors,omitempty"`
}

type Facet struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

type Facets struct {
	EventNames     []Facet `json:"eventNames"`
	AggregateNames []Facet `json:"aggregateNames"`
}

type eventMetadata struct {
	ID            uuid.UUID
	Name          string
	AggregateName string
	Time          time.Time
}

type EventPage struct {
	Items      []Event `json:"items"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

type Stream struct {
	AggregateName string    `json:"aggregateName"`
	AggregateID   string    `json:"aggregateId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type StreamPage struct {
	Items      []Stream `json:"items"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

type streamMetadata struct {
	AggregateName string
	AggregateID   uuid.UUID
	CreatedAt     time.Time
}

type EventFilter struct {
	Names          []string
	AggregateNames []string
	AggregateID    string
	From           *time.Time
	To             *time.Time
	Cursor         string
	Limit          int
}

type StreamFilter struct {
	AggregateNames []string
	AggregateID    string
	Cursor         string
	Limit          int
}

type Reader interface {
	EnsureIndexes(context.Context) error
	Ping(context.Context) error
	Summary(context.Context) (Summary, error)
	Facets(context.Context) (Facets, error)
	EventMetadata(context.Context, time.Time) ([]eventMetadata, error)
	Events(context.Context, EventFilter) (EventPage, error)
	Event(context.Context, string) (Event, error)
	StreamStarts(context.Context, time.Time) ([]streamMetadata, error)
	StreamEvents(context.Context, string, string, int, int) (EventPage, error)
	Close()
}

type eventCursor struct {
	TimeNano int64 `json:"t"`
}

type streamCursor struct {
	TimeNano int64 `json:"t"`
}

type versionCursor struct {
	Version int `json:"v"`
}

func encodeCursor(value any) string {
	b, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(raw string, target any) error {
	if raw == "" {
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return fmt.Errorf("decode cursor: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return fmt.Errorf("decode cursor: %w", err)
	}
	return nil
}

func decodeEventData(decoder Decoder, eventName string, raw []byte) (any, string) {
	if len(raw) == 0 {
		return nil, "event payload is empty"
	}
	data, err := decoder.Unmarshal(raw, eventName)
	if err != nil {
		return nil, err.Error()
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Sprintf("encode decoded payload as JSON: %s", err)
	}
	normalized, err := (JSONDecoder{}).Unmarshal(encoded, eventName)
	if err != nil {
		return nil, fmt.Sprintf("normalize decoded payload as JSON: %s", err)
	}
	return normalized, ""
}

func normalizedLimit(limit int) int {
	if limit <= 0 {
		return defaultPageSize
	}
	if limit > maxPageSize {
		return maxPageSize
	}
	return limit
}
