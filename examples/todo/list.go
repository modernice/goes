package todo

import (
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/modernice/goes/aggregate"
	"github.com/modernice/goes/event"
)

// ListAggregate is the name of the List aggregate.
const ListAggregate = "todo.list"

// List is a "todo" list.
type List struct {
	*aggregate.Base

	tasks   []string
	archive []string
}

// New returns the "todo" list with the given id.
func New(id uuid.UUID) *List {
	list := &List{Base: aggregate.New(ListAggregate, id)}

	// Register the event appliers for each of the aggregate events.
	event.ApplyWith(list, TaskAdded, list.add)
	event.ApplyWith(list, TaskRemoved, list.remove)
	event.ApplyWith(list, TasksDone, list.done)

	return list
}

// Tasks returns the active tasks.
func (list *List) Tasks() []string {
	return list.tasks
}

// Archive returns the completed tasks.
func (list *List) Archive() []string {
	return list.archive
}

// Contains returns whether the list contains the given task (case-insensitive).
func (list *List) Contains(task string) bool {
	task = strings.ToLower(task)
	for _, t := range list.tasks {
		if strings.ToLower(t) == task {
			return true
		}
	}
	return false
}

// Add adds the given task to the list, if it doesn't contain the task yet.
func (list *List) Add(task string) error {
	for _, t := range list.tasks {
		if strings.ToLower(t) == strings.ToLower(task) {
			return nil
		}
	}

	aggregate.Next(list, TaskAdded, task)

	return nil
}

func (list *List) add(evt event.Of[string]) {
	list.tasks = append(list.tasks, evt.Data())
}

// Remove removes the given task from the list.
func (list *List) Remove(task string) error {
	if !list.Contains(task) {
		return nil
	}
	aggregate.Next(list, TaskRemoved, TaskRemovedEvent{task})
	return nil
}

func (list *List) remove(evt event.Of[TaskRemovedEvent]) {
	for i, task := range list.tasks {
		if strings.ToLower(task) == strings.ToLower(evt.Data().Task) {
			list.tasks = append(list.tasks[:i], list.tasks[i+1:]...)
			return
		}
	}
}

// Done marks the given tasks as done.
func (list *List) Done(tasks ...string) error {
	if len(tasks) == 0 {
		return nil
	}

	var done []string
	for _, task := range tasks {
		ltask := strings.ToLower(task)
		for _, t := range list.tasks {
			if strings.ToLower(t) == ltask {
				done = append(done, task)
				break
			}
		}
	}

	if len(done) > 0 {
		aggregate.Next(list, TasksDone, done)
	}

	return nil
}

func (list *List) done(evt event.Of[[]string]) {
	for _, task := range evt.Data() {
		ltask := strings.ToLower(task)

		for i, t := range list.tasks {
			if strings.ToLower(t) == ltask {
				list.tasks = append(list.tasks[:i], list.tasks[i+1:]...)
				list.archive = append(list.archive, task)
				break
			}
		}
	}
}

func (list *List) print() {
	log.Printf("[List:%s] Tasks: %v", list.ID, list.Tasks())
	log.Printf("[List:%s] Archive: %v", list.ID, list.Archive())
}
