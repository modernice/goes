package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

var (
	// ErrInvalidRef is returned when providing an invalid aggregate reference
	// to a Grant() or Revoke() call.
	ErrInvalidRef = errors.New("invalid aggregate reference")
)

// Actions is a map that stores granted permissions:
//	map[AGGREGATE]map[ACTION]GRANT_COUNT
// Within the Actor and Role aggregates, GRANT_COUNT is always either 0 or 1.
type Actions map[aggregate.Ref]map[string]int

var allAggregatesWildcard = aggregate.Ref{
	Name: "*",
	ID:   uuid.Nil,
}

func (a Actions) allows(action string, ref aggregate.Ref) bool {
	return a.allowsActionWildcard(action, ref) || a.allowsWildcard(action, ref)
}

func (a Actions) allowsWildcard(action string, ref aggregate.Ref) bool {
	return a.allowsActionWildcard(action, allAggregatesWildcard) || // all aggregates, all ids, all action
		a.allowsActionWildcard(action, aggregate.Ref{ // all aggregates, single id, all actions
			Name: "*",
			ID:   ref.ID,
		}) ||
		a.allowsActionWildcard(action, aggregate.Ref{ // single aggregate, all ids, all actions
			Name: ref.Name,
			ID:   uuid.Nil,
		})
}

func (a Actions) allowsActionWildcard(action string, ref aggregate.Ref) bool {
	return a[ref][action] > 0 || a[ref]["*"] > 0
}

func (a Actions) granted(evt event.Of[PermissionGrantedData]) {
	data := evt.Data()
	perms, ok := a[data.Aggregate]
	if !ok {
		perms = make(map[string]int)
		a[data.Aggregate] = perms
	}
	for _, action := range data.Actions {
		perms[action]++
	}
}

func (a Actions) revoked(evt event.Of[PermissionRevokedData]) {
	data := evt.Data()
	perms, ok := a[data.Aggregate]
	if !ok {
		return
	}
	for _, action := range data.Actions {
		perms[action]--
		if (perms[action]) <= 0 {
			delete(perms, action)
		}
	}
}

func validateRef(ref aggregate.Ref) error {
	if strings.TrimSpace(ref.Name) == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidRef)
	}
	return nil
}
