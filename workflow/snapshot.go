package workflow

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// baseSnapshot is the encoded snapshot state of a Base.
type baseSnapshot struct {
	Status   Status
	Reason   string
	Triggers []uuid.UUID
	Commands []CommandRequestedData
	Timeouts []TimeoutRequestedData
}

// MarshalSnapshot implements snapshot.Marshaler. It deterministically
// encodes the lifecycle and effect state of the workflow, so that
// snapshot-enabled repositories (see WithRepository) can fetch workflows
// without replaying their full event history.
//
// Workflows that carry state of their own must implement MarshalSnapshot and
// UnmarshalSnapshot themselves and include the Base state:
//
//	func (w *OrderWorkflow) MarshalSnapshot() ([]byte, error) {
//		base, err := w.Base.MarshalSnapshot()
//		if err != nil {
//			return nil, err
//		}
//		return marshal(orderSnapshot{Base: base, Items: w.items})
//	}
//
//	func (w *OrderWorkflow) UnmarshalSnapshot(p []byte) error {
//		var snap orderSnapshot
//		if err := unmarshal(p, &snap); err != nil {
//			return err
//		}
//		w.items = snap.Items
//		return w.Base.UnmarshalSnapshot(snap.Base)
//	}
func (b *Base) MarshalSnapshot() ([]byte, error) {
	snap := baseSnapshot{
		Status:   b.status,
		Reason:   b.reason,
		Triggers: make([]uuid.UUID, 0, len(b.triggers)),
		Commands: make([]CommandRequestedData, 0, len(b.commands)),
		Timeouts: make([]TimeoutRequestedData, 0, len(b.timeoutsByKey)),
	}

	for id := range b.triggers {
		snap.Triggers = append(snap.Triggers, id)
	}
	sort.Slice(snap.Triggers, func(i, j int) bool {
		return bytes.Compare(snap.Triggers[i][:], snap.Triggers[j][:]) < 0
	})

	for _, data := range b.commands {
		snap.Commands = append(snap.Commands, data)
	}
	sort.Slice(snap.Commands, func(i, j int) bool {
		return bytes.Compare(snap.Commands[i].EffectID[:], snap.Commands[j].EffectID[:]) < 0
	})

	for _, data := range b.timeoutsByKey {
		snap.Timeouts = append(snap.Timeouts, data)
	}
	sort.Slice(snap.Timeouts, func(i, j int) bool {
		return snap.Timeouts[i].Key < snap.Timeouts[j].Key
	})

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(snap); err != nil {
		return nil, fmt.Errorf("encode workflow snapshot: %w", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalSnapshot implements snapshot.Unmarshaler. It restores the
// lifecycle and effect state of the workflow from a snapshot created by
// MarshalSnapshot.
func (b *Base) UnmarshalSnapshot(p []byte) error {
	var snap baseSnapshot
	if err := gob.NewDecoder(bytes.NewReader(p)).Decode(&snap); err != nil {
		return fmt.Errorf("decode workflow snapshot: %w", err)
	}

	b.status = snap.Status
	b.reason = snap.Reason

	b.triggers = make(map[uuid.UUID]struct{}, len(snap.Triggers))
	for _, id := range snap.Triggers {
		b.triggers[id] = struct{}{}
	}

	b.commands = make(map[uuid.UUID]CommandRequestedData, len(snap.Commands))
	for _, data := range snap.Commands {
		b.commands[data.EffectID] = data
	}

	b.timeoutsByKey = make(map[string]TimeoutRequestedData, len(snap.Timeouts))
	b.timeoutsByID = make(map[uuid.UUID]TimeoutRequestedData, len(snap.Timeouts))
	for _, data := range snap.Timeouts {
		b.timeoutsByKey[data.Key] = data
		b.timeoutsByID[data.EffectID] = data
	}

	return nil
}
