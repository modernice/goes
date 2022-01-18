package todo

import (
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
	return &List{
		Base: aggregate.New(ListAggregate, id),
	}
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

	aggregate.NextEvent(list, TaskAdded, TaskAddedEvent{task})

	return nil
}

func (list *List) add(evt event.EventOf[TaskAddedEvent]) {
	list.tasks = append(list.tasks, evt.Data().Task)
}

// Remove removes the given task from the list.
func (list *List) Remove(task string) error {
	if !list.Contains(task) {
		return nil
	}
	aggregate.NextEvent(list, TaskRemoved, TaskRemovedEvent{task})
	return nil
}

func (list *List) remove(evt event.EventOf[TaskRemovedEvent]) {
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
		aggregate.NextEvent(list, TaskDone, TaskDoneEvent{done})
	}

	return nil
}

func (list *List) done(evt event.EventOf[TaskDoneEvent]) {
	for _, task := range evt.Data().Tasks {
		task = strings.ToLower(task)

		for i, t := range list.tasks {
			if strings.ToLower(t) == task {
				list.tasks = append(list.tasks[:i], list.tasks[i+1:]...)
				list.archive = append(list.archive, task)
				break
			}
		}
	}
}

// ApplyEvent implements aggregate.Aggregate.
func (list *List) ApplyEvent(evt event.Event) {
	switch evt.Name() {
	case TaskAdded:
		list.add(event.Cast[TaskAddedEvent](evt))
	case TaskRemoved:
		list.remove(event.Cast[TaskRemovedEvent](evt))
	case TaskDone:
		list.done(event.Cast[TaskDoneEvent](evt))
	}
}
