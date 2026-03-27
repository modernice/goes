package natsjs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modernice/goes/codec"
	"github.com/modernice/goes/event"
)

type envelope struct {
	ID               uuid.UUID       `json:"id"`
	Name             string          `json:"name"`
	TimeNano         int64           `json:"time"` // Unix nanoseconds for lossless roundtrip.
	Data             json.RawMessage `json:"data"`
	AggregateName    string          `json:"agg_name,omitempty"`
	AggregateID      uuid.UUID       `json:"agg_id,omitempty"`
	AggregateVersion int             `json:"agg_version,omitempty"`
}

func marshalEnvelope(enc codec.Encoding, evt event.Event) ([]byte, error) {
	data, err := enc.Marshal(evt.Data())
	if err != nil {
		return nil, fmt.Errorf("marshal event data: %w", err)
	}

	aggID, aggName, aggVersion := evt.Aggregate()

	env := envelope{
		ID:               evt.ID(),
		Name:             evt.Name(),
		TimeNano:         evt.Time().UnixNano(),
		Data:             data,
		AggregateName:    aggName,
		AggregateID:      aggID,
		AggregateVersion: aggVersion,
	}

	b, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}

	return b, nil
}

func unmarshalEnvelope(enc codec.Encoding, data []byte) (event.Event, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	evtData, err := enc.Unmarshal(env.Data, env.Name)
	if err != nil {
		return nil, fmt.Errorf("unmarshal event data: %w", err)
	}

	opts := []event.Option{
		event.ID(env.ID),
		event.Time(time.Unix(0, env.TimeNano)),
	}

	if env.AggregateName != "" || env.AggregateID != uuid.Nil {
		opts = append(opts, event.Aggregate(env.AggregateID, env.AggregateName, env.AggregateVersion))
	}

	return event.New(env.Name, evtData, opts...), nil
}
