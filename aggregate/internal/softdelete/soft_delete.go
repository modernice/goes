package softdelete

import (
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// SoftDeleted returns whether the aggregate that is made up of the the provided
// events should be considered soft-deleted.
func SoftDeleted(events []event.Event) bool {
	var softDeleted bool
	for _, evt := range events {
		data := evt.Data()
		if data, ok := data.(aggregate.SoftDeleter); ok && data.SoftDelete() {
			softDeleted = true
		}
		if data, ok := data.(aggregate.SoftRestorer); ok && data.SoftRestore() {
			softDeleted = false
		}
	}
	return softDeleted
}
